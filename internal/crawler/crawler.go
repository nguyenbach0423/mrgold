package crawler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mrgold/internal/httpclient"
	"github.com/rs/zerolog/log"
)

type Crawler struct {
	httpClient *httpclient.Client
	Boards     map[string]*GoldPriceBoard
}

func NewCrawler(opts ...func(*Crawler)) *Crawler {
	c := &Crawler{
		httpClient: httpclient.NewClient(),
		Boards:     make(map[string]*GoldPriceBoard),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithHTTPClient(httpClient *httpclient.Client) func(*Crawler) {
	return func(c *Crawler) {
		c.httpClient = httpClient
	}
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

func (c *Crawler) Crawl() {
	go c.crawlSJC()
	go c.crawlDOJI()
	go c.crawlPNJ()
	go c.crawlBTMC()
	go c.crawlBTMH()
}

func (c *Crawler) crawlSJC() {
	resp, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodGet,
			"https://sjc.com.vn/GoldPrice/Services/PriceService.ashx",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
		),
	)

	if ok {
		var goldPriceBoard struct {
			UpdatedAt string `json:"latestDate"`
			Data      []struct {
				Id        int    `json:"Id"`
				Branch    string `json:"BranchName"`
				Name      string `json:"TypeName"`
				BuyPrice  string `json:"Buy"`
				SellPrice string `json:"Sell"`
			} `json:"data"`
		}

		if err := json.Unmarshal(resp.Body, &goldPriceBoard); err != nil {
			log.Error().Err(err).Send()
			return
		}

		var golds []Gold

		for _, gold := range goldPriceBoard.Data {
			if gold.Branch != "Hồ Chí Minh" || gold.Id == 129 || gold.Id == 210 {
				continue
			}
			golds = append(golds, Gold{
				Name:      gold.Name,
				BuyPrice:  convertStringPrice(gold.BuyPrice),
				SellPrice: convertStringPrice(gold.SellPrice),
			})
		}

		c.Boards["sjc"] = &GoldPriceBoard{
			Brand:     "SJC",
			Golds:     golds,
			UpdatedAt: goldPriceBoard.UpdatedAt,
		}
	}
}

func (c *Crawler) crawlDOJI() {
	resp, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodGet,
			"https://giavang.doji.vn/",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
			httpclient.WithQueryParams(map[string][]string{
				"q": {"doji/get/json/gia_vang_quoc_te"},
			}),
		),
	)

	if ok {
		respBody := resp.Body

		respBody = bytes.TrimPrefix(respBody, []byte("\xef\xbb\xbf"))
		respBody = bytes.ReplaceAll(respBody, []byte(`\x3c`), []byte("<"))
		respBody = bytes.ReplaceAll(respBody, []byte(`\x3e`), []byte(">"))

		var goldPriceBoard struct {
			Data string `json:"main_price"`
		}

		if err := json.Unmarshal(respBody, &goldPriceBoard); err != nil {
			log.Error().Err(err).Send()
			return
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(goldPriceBoard.Data))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}

		var golds []Gold

		doc.Find("table.goldprice-view tbody tr").Each(func(_ int, s *goquery.Selection) {
			gold := Gold{}

			name := strings.TrimSpace(s.Find("td.first span.title").Text())
			gold.Name = convertGoldNameDOJI(name)
			if gold.Name == "" {
				return
			}

			gold.BuyPrice = convertStringPrice(strings.TrimSpace(s.Find("td.goldprice-td-0 div.item-relative").Text()))
			gold.SellPrice = convertStringPrice(strings.TrimSpace(s.Find("td.goldprice-td-1 div.item-relative").Text()))

			golds = append(golds, gold)
		})

		updatedAt := strings.TrimSpace(doc.Find("span.update-time").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "Cập nhập lúc: ", "")

		c.Boards["doji"] = &GoldPriceBoard{
			Brand:     "DOJI",
			Golds:     golds,
			UpdatedAt: updatedAt,
		}
	}
}

func (c *Crawler) crawlPNJ() {
	resp, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodGet,
			"https://edge-api.pnj.io/ecom-frontend/v1/get-gold-price",
			httpclient.WithQueryParams(map[string][]string{
				"zone": {"11"},
			}),
			httpclient.WithHeaders(httpclient.DefaultHeaders),
		),
	)

	if ok {
		var goldPriceBoard struct {
			Data []struct {
				Code      string `json:"masp"`
				Name      string `json:"tensp"`
				BuyPrice  int    `json:"giamua"`
				SellPrice int    `json:"giaban"`
			} `json:"data"`
			UpdatedAt string `json:"updateDate"`
		}

		if err := json.Unmarshal(resp.Body, &goldPriceBoard); err != nil {
			log.Error().Err(err).Send()
			return
		}

		var golds []Gold

		codes := []string{"SJC", "N24K", "KB", "TL", "PNJ", "24K", "999", "99", "75", "58.5", "41"}
		for _, gold := range goldPriceBoard.Data {
			if !slices.Contains(codes, gold.Code) {
				continue
			}

			golds = append(golds, Gold{
				Name:      gold.Name,
				BuyPrice:  convertNumericPrice(gold.BuyPrice),
				SellPrice: convertNumericPrice(gold.SellPrice),
			})
		}

		t, _ := time.Parse("02/01/2006 15:04:05", goldPriceBoard.UpdatedAt)

		c.Boards["pnj"] = &GoldPriceBoard{
			Brand:     "PNJ",
			Golds:     golds,
			UpdatedAt: t.Format("15:04 02/01/2006"),
		}
	}
}

func (c *Crawler) crawlBTMC() {
	resp, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodGet,
			"https://btmc.vn/bieu-do-gia-vang.html?t=ngay&srsltid=AfmBOopkLFTaGSDib4E6WuWUNcG1Z5Q9vmfqzNBuJUHwlCoYX66i8HPl",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
		),
	)

	if ok {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body))
		if err != nil {
			log.Error().Err(err).Send()
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

			if strings.Contains(strings.ToLower(name), "nguyên liệu") {
				return
			}

			gold := Gold{
				Name:      convertGoldNameBTMC(name),
				BuyPrice:  convertStringPrice(buyPrice),
				SellPrice: convertStringPrice(sellPrice),
			}

			golds = append(golds, gold)
		})

		updatedAt := strings.TrimSpace(doc.Find("p.note span").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "Cập nhật lúc ", "")

		t, _ := time.Parse("02/01/2006 15:04", updatedAt)

		c.Boards["btmc"] = &GoldPriceBoard{
			Brand:     "Bảo Tín Minh Châu",
			Golds:     golds,
			UpdatedAt: t.Format("15:04 02/01/2006"),
		}
	}
}

func (c *Crawler) crawlBTMH() {
	resp, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodGet,
			"https://baotinmanhhai.vn/gia-vang-hom-nay",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
		),
	)

	if ok {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}

		var golds []Gold

		doc.Find("table.gold-table-content tbody tr").Each(func(_ int, s *goquery.Selection) {
			var cells []string

			s.Find("td").Each(func(_ int, s *goquery.Selection) {
				cells = append(cells, strings.TrimSpace(s.Text()))
			})

			if len(cells) == 0 {
				return
			}

			gold := Gold{
				Name:      cells[0],
				BuyPrice:  cells[1],
				SellPrice: cells[2],
			}

			golds = append(golds, gold)
		})

		updatedAt := strings.TrimSpace(doc.Find("p.note").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "(Cập nhật lúc ", "")
		updatedAt = strings.ReplaceAll(updatedAt, ") (đơn vị tính: đồng/chỉ)", "")

		c.Boards["btmh"] = &GoldPriceBoard{
			Brand:     "Bảo Tín Mạnh Hải",
			Golds:     golds,
			UpdatedAt: updatedAt,
		}
	}
}

func convertNumericPrice(n int) string {
	n = n * 1000

	s := strconv.Itoa(n)
	var result []string

	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}

	result = append([]string{s}, result...)
	return strings.Join(result, ".")
}

func convertStringPrice(s string) string {
	if s == "" {
		return s
	}

	s = strings.ReplaceAll(s, ",", "")

	n, err := strconv.Atoi(s)
	if err != nil {
		log.Error().Err(err).Send()
		return s
	}
	n = n * 1000

	s = strconv.Itoa(n)
	var result []string

	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}

	result = append([]string{s}, result...)
	return strings.Join(result, ".")
}

func convertGoldNameDOJI(s string) string {
	s = strings.ToLower(s)
	if strings.Contains(s, "nguyên liệu") {
		return ""
	}

	s = strings.ToUpper(string(s[0])) + s[1:]
	s = strings.ReplaceAll(s, "Avpl/sjc", "AVPL/SJC")
	s = strings.ReplaceAll(s, "hưng thịnh vượng", "Hưng Thịnh Vượng")
	s = strings.ReplaceAll(s, " - bán lẻ", "")

	return s
}

func convertGoldNameBTMC(s string) string {
	s = strings.ToLower(s)
	s = strings.ToUpper(string(s[0])) + s[1:]
	s = strings.ReplaceAll(s, "vrtl", "VRTL")
	s = strings.ReplaceAll(s, "bảo tín minh châu", "Bảo Tín Minh Châu")
	s = strings.ReplaceAll(s, "sjc", "SJC")
	s = strings.ReplaceAll(s, "vàng rồng thăng long", "Vàng Rồng Thăng Long")

	return s
}
