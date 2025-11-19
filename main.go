package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mrgold/internal/crawler"
	"github.com/mrgold/internal/httpclient"
	"github.com/mrgold/internal/httpserver"
	"github.com/mrgold/internal/telegram"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//	"https://btmc.vn/bieu-do-gia-vang.html?t=ngay&srsltid=AfmBOopkLFTaGSDib4E6WuWUNcG1Z5Q9vmfqzNBuJUHwlCoYX66i8HPl"
//	"https://baotinmanhhai.vn/gia-vang-hom-nay"

func main() {
	httpClient := httpclient.NewClient(
		httpclient.WithTimeout(5*time.Second),
		httpclient.WithRetryConfig(&httpclient.RetryConfig{
			MaxRetries: 3,
			Backoff:    200 * time.Millisecond,
			MaxBackoff: 1 * time.Second,
		}),
	)

	c := crawler.NewCrawler(
		crawler.WithHTTPClient(httpClient),
	)

	telegramClient := telegram.NewClient(
		telegram.WithHTTPClient(httpClient),
		telegram.WithBaseURL(os.Getenv("TELEGRAM_BOT_BASE_URL")),
		telegram.WithCrawler(c),
	)

	if ok := telegramClient.SetWebhook(os.Getenv("PUBLIC_DOMAIN")); !ok {
		os.Exit(1)
	}

	if ok := telegramClient.SetMyCommands(); !ok {
		os.Exit(1)
	}

	log.Info().Msg("telegram bot was configured")

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			c.Crawl()

			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Minute):
			}
		}
	}()

	s := httpserver.NewServer(
		os.Getenv("PORT"),
		httpserver.WithTelegramClient(telegramClient),
	)

	go func() {
		if ok := s.Start(); !ok {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	cancel()
	wg.Wait()

	if ok := s.Stop(); !ok {
		os.Exit(1)
	}
}

func init() {
	zerolog.TimeFieldFormat = time.DateTime
}

//func crawlPNJ() {
//	resp, ok := doRequest(http.MethodGet, "https://edge-api.pnj.io/ecom-frontend/v1/get-gold-price", map[string]string{
//		"zone": "11",
//	}, nil)
//
//	if ok {
//		var goldPriceBoard struct {
//			Data []struct {
//				Code      string `json:"masp"`
//				Name      string `json:"tensp"`
//				BuyPrice  int    `json:"giamua"`
//				SellPrice int    `json:"giaban"`
//			} `json:"data"`
//			Branch    string `json:"chinhanh"`
//			UpdatedAt string `json:"updateDate"`
//		}
//
//		if err := json.Unmarshal(resp, &goldPriceBoard); err != nil {
//			log.Error().Err(err).Send()
//			return
//		}
//
//		var golds []Gold
//
//		for _, gold := range goldPriceBoard.Data {
//			golds = append(golds, Gold{
//				Name:      gold.Name,
//				BuyPrice:  strconv.Itoa(gold.BuyPrice),
//				SellPrice: strconv.Itoa(gold.SellPrice),
//			})
//		}
//
//		location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
//		if err != nil {
//			log.Error().Err(err).Send()
//			return
//		}
//		updatedAt := time.Now().In(location).Format(time.DateTime)
//
//		goldPriceBoards["pnj"] = &GoldPriceBoard{
//			Brand:     "PNJ",
//			Golds:     golds,
//			UpdatedAt: updatedAt,
//		}
//	}
//}
//
//func crawlBTMC(r *colly.Response) {
//	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
//	if err != nil {
//		log.Error().Err(err).Str("url", r.Request.URL.String()).Send()
//		return
//	}
//
//	var golds []Gold
//
//	doc.Find("table.bd_price_home tr").Each(func(_ int, s *goquery.Selection) {
//		var cells []string
//
//		s.Find("td").Each(func(_ int, s *goquery.Selection) {
//			cells = append(cells, strings.TrimSpace(s.Text()))
//		})
//
//		if len(cells) == 0 {
//			return
//		}
//
//		var name = ""
//		var buyPrice = ""
//		var sellPrice = ""
//
//		pattern := regexp.MustCompile("^[0-9]+$")
//
//		if len(cells) == 5 {
//			name = strings.ReplaceAll(cells[1], "  ", " ")
//			if pattern.MatchString(cells[3]) {
//				buyPrice = cells[3]
//			}
//			if pattern.MatchString(cells[4]) {
//				sellPrice = cells[4]
//			}
//		} else if len(cells) == 4 {
//			name = strings.ReplaceAll(cells[0], "  ", " ")
//			if pattern.MatchString(cells[2]) {
//				buyPrice = cells[2]
//			}
//			if pattern.MatchString(cells[3]) {
//				sellPrice = cells[3]
//			}
//		}
//
//		golds = append(golds, Gold{
//			Name:      name,
//			BuyPrice:  buyPrice,
//			SellPrice: sellPrice,
//		})
//	})
//
//	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
//	if err != nil {
//		log.Error().Err(err).Send()
//		return
//	}
//	updatedAt := time.Now().In(location).Format(time.DateTime)
//
//	goldPriceBoards["btmc"] = &GoldPriceBoard{
//		Brand:     "BẢO TÍN MINH CHÂU",
//		Golds:     golds,
//		UpdatedAt: updatedAt,
//	}
//}
//
//func crawlBTMH(r *colly.Response) {
//	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
//	if err != nil {
//		log.Error().Err(err).Str("url", r.Request.URL.String()).Send()
//		return
//	}
//
//	var golds []Gold
//
//	doc.Find("table.gold-table-content tr").Each(func(_ int, s *goquery.Selection) {
//		var cells []string
//
//		s.Find("td").Each(func(_ int, s *goquery.Selection) {
//			cells = append(cells, strings.TrimSpace(s.Text()))
//		})
//
//		if len(cells) == 0 {
//			return
//		}
//
//		golds = append(golds, Gold{
//			Name:      cells[0],
//			BuyPrice:  cells[1],
//			SellPrice: cells[2],
//		})
//	})
//
//	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
//	if err != nil {
//		log.Error().Err(err).Send()
//		return
//	}
//	updatedAt := time.Now().In(location).Format(time.DateTime)
//
//	goldPriceBoards["btmh"] = &GoldPriceBoard{
//		Brand:     "BẢO TÍN MẠNH HẢI",
//		Golds:     golds,
//		UpdatedAt: updatedAt,
//	}
//}
