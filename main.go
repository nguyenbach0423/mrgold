package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const userAgent = "MrGoldBot/1.0 (+https://t.me/mr_g0ld_bot)"

var config *Config

var crawlers = map[string]*Crawler{
	"sjc":  {APIFunc: crawlSJC, CollyFunc: nil},
	"doji": {APIFunc: crawlDOJI, CollyFunc: nil},
	"pnj":  {APIFunc: crawlPNJ, CollyFunc: nil},
	"https://btmc.vn/bieu-do-gia-vang.html?t=ngay&srsltid=AfmBOopkLFTaGSDib4E6WuWUNcG1Z5Q9vmfqzNBuJUHwlCoYX66i8HPl": {APIFunc: nil, CollyFunc: crawlBTMC},
	"https://baotinmanhhai.vn/gia-vang-hom-nay": {APIFunc: nil, CollyFunc: crawlBTMH},
}

var goldPriceBoards = make(map[string]*GoldPriceBoard)

func main() {
	zerolog.TimeFieldFormat = time.DateTime

	loadConfig()

	var err error

	_, ok := doRequest(http.MethodPost, config.TelegramBotBaseURL+"/setWebhook", map[string]string{
		"url": config.PublicDomain + "/webhook",
	})
	if !ok {
		os.Exit(1)
	}

	c := colly.NewCollector(
		colly.UserAgent(userAgent),
		colly.MaxDepth(1),
		colly.Async(true),
		colly.AllowURLRevisit(),
	)

	err = c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       2 * time.Second,
		RandomDelay: 2 * time.Second,
		Parallelism: 5,
	})

	if err != nil {
		log.Error().Err(err).Send()
		os.Exit(1)
	}

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "*/*")
	})

	c.OnResponse(func(r *colly.Response) {
		crawler := crawlers[r.Request.URL.String()]
		crawler.CollyFunc(r)
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Error().Err(err).Str("url", r.Request.URL.String()).Send()
	})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			for crawlURL, crawler := range crawlers {
				if crawler.APIFunc != nil {
					go crawler.APIFunc()
				} else {
					if err = c.Visit(crawlURL); err != nil {
						log.Error().Err(err).Str("url", crawlURL).Send()
					}
				}
			}

			c.Wait()

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
		}
	}()

	log.Info().Msg("server is listening on port " + config.Port)

	s := http.Server{
		Addr:    ":" + config.Port,
		Handler: &Handler{},
	}

	go func() {
		if err = s.ListenAndServe(); !errors.Is(http.ErrServerClosed, err) {
			log.Error().Err(err).Send()
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err = s.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Send()
		os.Exit(1)
	}

	cancel()

	wg.Wait()
}

type Config struct {
	Port               string
	PublicDomain       string
	TelegramBotBaseURL string
}

func loadConfig() {
	config = &Config{
		Port:               os.Getenv("PORT"),
		PublicDomain:       os.Getenv("PUBLIC_DOMAIN"),
		TelegramBotBaseURL: os.Getenv("TELEGRAM_BOT_BASE_URL"),
	}
}

type APIFunc func()

type CollyFunc func(*colly.Response)

type Crawler struct {
	APIFunc   APIFunc
	CollyFunc CollyFunc
}

type Gold struct {
	Name      string
	BuyPrice  string
	SellPrice string
}

type GoldPriceBoard struct {
	Brand     string
	Golds     []Gold
	UpdatedAt string
}

func (gpb *GoldPriceBoard) Display() string {
	var builder strings.Builder

	builder.WriteString("<b>BẢNG GIÁ VÀNG " + gpb.Brand + "</b>\n")
	builder.WriteString("\n")
	for _, gold := range gpb.Golds {
		if gold.BuyPrice == "" && gold.SellPrice == "" {
			continue
		} else if gold.BuyPrice == "" {
			builder.WriteString("• <b>" + gold.Name + "</b> - Bán ra: <b>" + gold.SellPrice + "</b>\n")
		} else if gold.SellPrice == "" {
			builder.WriteString("• <b>" + gold.Name + "</b> - Mua vào: <b>" + gold.BuyPrice + "</b>\n")
		} else {
			builder.WriteString("• <b>" + gold.Name + "</b> - Mua vào: <b>" + gold.BuyPrice + "</b> - Bán ra: <b>" + gold.SellPrice + "</b>\n")
		}
	}
	builder.WriteString("\n")
	builder.WriteString("Giá vàng cập nhật lúc: <b>" + gpb.UpdatedAt + "</b>\n")

	return builder.String()
}

func crawlSJC() {
	resp, ok := doRequest(http.MethodGet, "https://sjc.com.vn/GoldPrice/Services/PriceService.ashx", nil)

	if ok {
		var goldPriceBoard struct {
			Data []struct {
				Branch    string `json:"BranchName"`
				Name      string `json:"TypeName"`
				BuyPrice  string `json:"Buy"`
				SellPrice string `json:"Sell"`
			} `json:"data"`
		}

		if err := json.Unmarshal(resp, &goldPriceBoard); err != nil {
			log.Error().Err(err).Send()
			return
		}

		var golds []Gold

		for _, gold := range goldPriceBoard.Data {
			if gold.Branch != "Hồ Chí Minh" {
				continue
			}
			golds = append(golds, Gold{
				Name:      gold.Name,
				BuyPrice:  gold.BuyPrice,
				SellPrice: gold.SellPrice,
			})
		}

		location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
		updatedAt := time.Now().In(location).Format(time.DateTime)

		goldPriceBoards["sjc"] = &GoldPriceBoard{
			Brand:     "SJC",
			Golds:     golds,
			UpdatedAt: updatedAt,
		}
	}
}

func crawlDOJI() {
	resp, ok := doRequest(http.MethodGet, "https://giavang.doji.vn/", map[string]string{
		"q": "doji/get/json/gia_vang_quoc_te",
	})

	resp = bytes.TrimPrefix(resp, []byte("\xef\xbb\xbf"))
	resp = bytes.ReplaceAll(resp, []byte(`\x3c`), []byte("<"))
	resp = bytes.ReplaceAll(resp, []byte(`\x3e`), []byte(">"))

	if ok {
		var goldPriceBoard struct {
			Error  int    `json:"error"`
			Data   string `json:"main_price"`
			Status int    `json:"status"`
		}

		if err := json.Unmarshal(resp, &goldPriceBoard); err != nil {
			log.Error().Err(err).Send()
			return
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(goldPriceBoard.Data))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}

		var golds []Gold

		doc.Find("table tbody tr").Each(func(_ int, s *goquery.Selection) {
			gold := Gold{}

			gold.Name = strings.TrimSpace(s.Find("td.first span.title").Text())
			gold.BuyPrice = strings.TrimSpace(s.Find("td.goldprice-td-0 div").Text())
			gold.SellPrice = strings.TrimSpace(s.Find("td.goldprice-td-1 div").Text())

			golds = append(golds, gold)
		})

		location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
		updatedAt := time.Now().In(location).Format(time.DateTime)

		goldPriceBoards["doji"] = &GoldPriceBoard{
			Brand:     "DOJI",
			Golds:     golds,
			UpdatedAt: updatedAt,
		}
	}
}

func crawlPNJ() {
	resp, ok := doRequest(http.MethodGet, "https://edge-api.pnj.io/ecom-frontend/v1/get-gold-price", map[string]string{
		"zone": "11",
	})

	if ok {
		var goldPriceBoard struct {
			Data []struct {
				Code      string `json:"masp"`
				Name      string `json:"tensp"`
				BuyPrice  int    `json:"giaban"`
				SellPrice int    `json:"giamua"`
			} `json:"data"`
			Branch    string `json:"chinhanh"`
			UpdatedAt string `json:"updateDate"`
		}

		if err := json.Unmarshal(resp, &goldPriceBoard); err != nil {
			log.Error().Err(err).Send()
			return
		}

		var golds []Gold

		for _, gold := range goldPriceBoard.Data {
			golds = append(golds, Gold{
				Name:      gold.Name,
				BuyPrice:  strconv.Itoa(gold.BuyPrice),
				SellPrice: strconv.Itoa(gold.SellPrice),
			})
		}

		location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
		updatedAt := time.Now().In(location).Format(time.DateTime)

		goldPriceBoards["pnj"] = &GoldPriceBoard{
			Brand:     "PNJ",
			Golds:     golds,
			UpdatedAt: updatedAt,
		}
	}
}

func crawlBTMC(r *colly.Response) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
	if err != nil {
		log.Error().Err(err).Str("url", r.Request.URL.String()).Send()
		return
	}

	var golds []Gold

	doc.Find("table.bd_price_home tr").Each(func(_ int, s *goquery.Selection) {
		var cells []string

		s.Find("td").Each(func(_ int, s *goquery.Selection) {
			cells = append(cells, strings.TrimSpace(s.Text()))
		})

		if len(cells) == 0 {
			return
		}

		var name = ""
		var buyPrice = ""
		var sellPrice = ""

		pattern := regexp.MustCompile("^[0-9]+$")

		if len(cells) == 5 {
			name = strings.ReplaceAll(cells[1], "  ", " ")
			if pattern.MatchString(cells[3]) {
				buyPrice = cells[3]
			}
			if pattern.MatchString(cells[4]) {
				sellPrice = cells[4]
			}
		} else if len(cells) == 4 {
			name = strings.ReplaceAll(cells[0], "  ", " ")
			if pattern.MatchString(cells[2]) {
				buyPrice = cells[2]
			}
			if pattern.MatchString(cells[3]) {
				sellPrice = cells[3]
			}
		}

		golds = append(golds, Gold{
			Name:      name,
			BuyPrice:  buyPrice,
			SellPrice: sellPrice,
		})
	})

	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
	updatedAt := time.Now().In(location).Format(time.DateTime)

	goldPriceBoards["btmc"] = &GoldPriceBoard{
		Brand:     "BẢO TÍN MINH CHÂU",
		Golds:     golds,
		UpdatedAt: updatedAt,
	}
}

func crawlBTMH(r *colly.Response) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
	if err != nil {
		log.Error().Err(err).Str("url", r.Request.URL.String()).Send()
		return
	}

	var golds []Gold

	doc.Find("table.gold-table-content tr").Each(func(_ int, s *goquery.Selection) {
		var cells []string

		s.Find("td").Each(func(_ int, s *goquery.Selection) {
			cells = append(cells, strings.TrimSpace(s.Text()))
		})

		if len(cells) == 0 {
			return
		}

		golds = append(golds, Gold{
			Name:      cells[0],
			BuyPrice:  cells[1],
			SellPrice: cells[2],
		})
	})

	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
	updatedAt := time.Now().In(location).Format(time.DateTime)

	goldPriceBoards["btmh"] = &GoldPriceBoard{
		Brand:     "BẢO TÍN MẠNH HẢI",
		Golds:     golds,
		UpdatedAt: updatedAt,
	}
}

type Handler struct{}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Path
	if endpoint != "/webhook" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	method := r.Method
	if method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var err error

	var body []byte
	body, err = io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Send()

		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Info().
		Str("host", r.Host).
		Str("ip", getClientIP(r)).
		Str("method", method).
		Str("endpoint", endpoint).
		Str("userAgent", r.UserAgent()).
		Str("request", string(body)).
		Send()

	var payload struct {
		Message struct {
			Chat struct {
				Id int `json:"id"`
			} `json:"chat"`
			Text string `json:"text"`
		} `json:"message"`
	}

	if err = json.Unmarshal(body, &payload); err != nil {
		log.Error().Err(err).Send()

		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id := strconv.Itoa(payload.Message.Chat.Id)
	text := strings.ToLower(strings.TrimSpace(payload.Message.Text))

	var pattern = regexp.MustCompile("(?i)^/gold\\s+(SJC|PNJ|DOJI|BTMC|BTMH)$")

	if text == "" || !pattern.MatchString(text) {
		go sendMessage(id, "Vui lòng chọn một trong các thương hiệu sau:\n<code><b>/gold SJC</b></code> | <code><b>/gold DOJI</b></code> | <code><b>/gold PNJ</b></code> | <code><b>/gold BTMC</b></code> | <code><b>/gold BTMH</b></code>")
		return
	}

	symbol := strings.ToLower(strings.TrimSpace(text[6:]))

	goldPriceBoard, exist := goldPriceBoards[symbol]
	if !exist {
		go sendMessage(id, "Giá vàng hiện chưa được cập nhật. Vui lòng thử lại trong giây lát.")
		return
	}

	go sendMessage(id, goldPriceBoard.Display())
}

func getClientIP(r *http.Request) string {
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		return strings.Split(xForwardedFor, ",")[0]
	}

	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	return ip
}

func sendMessage(id, text string) {
	if text == "" {
		return
	}

	doRequest(http.MethodPost, config.TelegramBotBaseURL+"/sendMessage", map[string]string{
		"chat_id":    id,
		"text":       text,
		"parse_mode": "HTML",
	})
}

func doRequest(method, baseURL string, params map[string]string) ([]byte, bool) {
	var err error

	queryParams := url.Values{}

	for k, v := range params {
		queryParams.Add(k, v)
	}

	requestURL := baseURL + "?" + queryParams.Encode()

	var req *http.Request
	req, err = http.NewRequest(method, requestURL, nil)
	if err != nil {
		log.Error().Err(err).Send()
		return nil, false
	}

	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		log.Error().Err(err).Send()
		return nil, false
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Send()
		return nil, false
	}

	if resp.StatusCode != http.StatusOK {
		log.Warn().
			Str("method", req.Method).
			Str("endpoint", req.URL.Path).
			Str("query", req.URL.RawQuery).
			Str("status", resp.Status).
			Str("response", string(body)).
			Send()

		return body, false
	}

	return body, true
}
