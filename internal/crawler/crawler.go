package crawler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
			if gold.Branch != "Hồ Chí Minh" {
				continue
			}
			golds = append(golds, Gold{
				Name:      gold.Name,
				BuyPrice:  gold.BuyPrice,
				SellPrice: gold.SellPrice,
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

			gold.Name = strings.TrimSpace(s.Find("td.first span.title").Text())
			gold.BuyPrice = strings.TrimSpace(s.Find("td.goldprice-td-0 div.item-relative").Text())
			gold.SellPrice = strings.TrimSpace(s.Find("td.goldprice-td-1 div.item-relative").Text())

			golds = append(golds, gold)
		})

		updatedAt := strings.TrimSpace(doc.Find("span.update-time").Text())
		updatedAt = strings.TrimPrefix(updatedAt, "Cập nhập lúc: ")

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

		for _, gold := range goldPriceBoard.Data {
			golds = append(golds, Gold{
				Name:      gold.Name,
				BuyPrice:  strconv.Itoa(gold.BuyPrice),
				SellPrice: strconv.Itoa(gold.SellPrice),
			})
		}

		c.Boards["pnj"] = &GoldPriceBoard{
			Brand:     "PNJ",
			Golds:     golds,
			UpdatedAt: goldPriceBoard.UpdatedAt,
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
		updatedAt = strings.TrimPrefix(updatedAt, "Cập nhập lúc: ")

		c.Boards["btmh"] = &GoldPriceBoard{
			Brand:     "Bảo Tín Mạnh Hải",
			Golds:     golds,
			UpdatedAt: updatedAt,
		}
	}
}
