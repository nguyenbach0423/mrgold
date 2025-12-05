package crawler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mrgold/internal/httpclient"
	"github.com/mrgold/internal/storage"
	"github.com/mrgold/internal/store"
	"github.com/rs/zerolog/log"
)

type Crawler struct {
	mu         sync.RWMutex
	httpClient *httpclient.Client
	stg        *storage.Storage
	store      *store.Store
}

func NewCrawler(stg *storage.Storage, opts ...func(*Crawler)) *Crawler {
	c := &Crawler{
		httpClient: httpclient.NewClient(),
		stg:        stg,
		store:      store.NewStore(),
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

func WithStore(store *store.Store) func(*Crawler) {
	return func(c *Crawler) {
		c.store = store
	}
}

func (c *Crawler) Crawl() {
	go c.crawlSJC()
	go c.crawlDOJI()
	go c.crawlPNJ()
	go c.crawlBTMC()
	go c.crawlBTMH()
	go c.crawlSJCV2()
	go c.crawlDOJIV2()
	go c.crawlPNJV2()
	go c.crawlBTMCV2()
	go c.crawlBTMHV2()
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

		var golds []store.Gold

		idMatrix := map[int]string{
			1:   "01",
			17:  "02",
			33:  "03",
			49:  "04",
			65:  "05",
			81:  "06",
			97:  "07",
			113: "08",
			145: "09",
			161: "10",
		}

		for _, gold := range goldPriceBoard.Data {
			if gold.Branch != "Hồ Chí Minh" || gold.Id == 129 || gold.Id == 210 {
				continue
			}
			golds = append(golds, store.Gold{
				Code:      idMatrix[gold.Id],
				Name:      gold.Name,
				BuyPrice:  convertStringPrice(gold.BuyPrice),
				SellPrice: convertStringPrice(gold.SellPrice),
			})
		}

		c.store.SetBoard("sjc", &store.GoldPriceBoard{
			BrandName: "SJC",
			Golds:     golds,
			UpdatedAt: goldPriceBoard.UpdatedAt,
		})
	}
}

func (c *Crawler) crawlSJCV2() {
	resp, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodGet,
			"https://sjc.com.vn/GoldPrice/Services/PriceService.ashx",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
		),
	)

	if ok {
		var goldPriceBoard struct {
			UpdatedAt  string `json:"latestDate"`
			GoldPrices []struct {
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

		brandID, err := c.stg.GetBrandID("sjc")
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		var crawlMeta *storage.CrawlMeta
		crawlMeta, err = c.stg.GetCrawlMeta(brandID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		if crawlMeta.UpdatedAt == goldPriceBoard.UpdatedAt {
			return
		}

		goldCodes := map[int]string{
			1:   "sjc01",
			17:  "sjc02",
			33:  "sjc03",
			49:  "sjc04",
			65:  "sjc05",
			81:  "sjc06",
			97:  "sjc07",
			113: "sjc08",
			123: "sjc09",
			210: "sjc10",
			145: "sjc11",
			161: "sjc12",
		}

		goldIDs := c.stg.GetGoldIDs(brandID)
		if len(goldIDs) == 0 {
			return
		}

		var goldPrices []storage.GoldPrice

		for _, goldPrice := range goldPriceBoard.GoldPrices {
			if goldPrice.Branch != "Hồ Chí Minh" {
				continue
			}

			var exists bool

			var code string
			code, exists = goldCodes[goldPrice.Id]
			if !exists {
				continue
			}

			var id int
			id, exists = goldIDs[code]
			if !exists {
				continue
			}

			var buyPrice int
			var sellPrice int

			var buyPriceText string
			var sellPriceText string

			buyPrice, buyPriceText, err = convertPriceFromString(goldPrice.BuyPrice)
			if err != nil {
				continue
			}
			sellPrice, sellPriceText, err = convertPriceFromString(goldPrice.SellPrice)
			if err != nil {
				continue
			}

			goldPrices = append(goldPrices, storage.GoldPrice{
				GoldID:        id,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				BuyPriceText:  buyPriceText,
				SellPriceText: sellPriceText,
				UpdatedAt:     goldPriceBoard.UpdatedAt,
			})
		}

		if err = c.stg.SaveGoldPrices(goldPrices, brandID, goldPriceBoard.UpdatedAt); err != nil {
			log.Error().Err(err).Send()
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

		var golds []store.Gold

		idMatrix := map[string]string{
			"AVPL/SJC":                          "01",
			"Nhẫn tròn 9999 (Hưng Thịnh Vượng)": "02",
			"Nữ trang 9999":                     "03",
			"Nữ trang 999":                      "04",
		}

		doc.Find("table.goldprice-view tbody tr").Each(func(_ int, s *goquery.Selection) {
			gold := store.Gold{}

			name := strings.TrimSpace(s.Find("td.first span.title").Text())
			gold.Name = convertGoldNameDOJI(name)
			if gold.Name == "" {
				return
			}

			gold.Code = idMatrix[gold.Name]

			gold.BuyPrice = convertStringPrice(strings.TrimSpace(s.Find("td.goldprice-td-0 div.item-relative").Text()))
			gold.SellPrice = convertStringPrice(strings.TrimSpace(s.Find("td.goldprice-td-1 div.item-relative").Text()))

			golds = append(golds, gold)
		})

		updatedAt := strings.TrimSpace(doc.Find("span.update-time").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "Cập nhập lúc: ", "")

		c.store.SetBoard("doji", &store.GoldPriceBoard{
			BrandName: "DOJI",
			Golds:     golds,
			UpdatedAt: updatedAt,
		})
	}
}

func (c *Crawler) crawlDOJIV2() {
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

		goldCodes := map[string]string{
			"AVPL/SJC":                          "doji01",
			"Nhẫn tròn 9999 (Hưng Thịnh Vượng)": "doji02",
			"Nữ trang 9999":                     "doji03",
			"Nữ trang 999":                      "doji04",
		}

		var brandID int
		brandID, err = c.stg.GetBrandID("doji")
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		updatedAt := strings.TrimSpace(doc.Find("span.update-time").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "Cập nhập lúc: ", "")

		var crawlMeta *storage.CrawlMeta
		crawlMeta, err = c.stg.GetCrawlMeta(brandID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		if crawlMeta.UpdatedAt == updatedAt {
			return
		}

		goldIDs := c.stg.GetGoldIDs(brandID)
		if len(goldIDs) == 0 {
			return
		}

		var goldPrices []storage.GoldPrice

		doc.Find("table.goldprice-view tbody tr").Each(func(_ int, s *goquery.Selection) {
			name := strings.TrimSpace(s.Find("td.first span.title").Text())
			name = convertGoldNameDOJI(name)

			var exists bool

			var code string
			code, exists = goldCodes[name]
			if !exists {
				return
			}

			var id int
			id, exists = goldIDs[code]
			if !exists {
				return
			}

			var buyPrice int
			var sellPrice int

			var buyPriceText string
			var sellPriceText string

			buyPrice, buyPriceText, err = convertPriceFromString(strings.TrimSpace(s.Find("td.goldprice-td-0 div.item-relative").Text()))
			if err != nil {
				return
			}
			sellPrice, sellPriceText, err = convertPriceFromString(strings.TrimSpace(s.Find("td.goldprice-td-1 div.item-relative").Text()))
			if err != nil {
				return
			}

			goldPrices = append(goldPrices, storage.GoldPrice{
				GoldID:        id,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				BuyPriceText:  buyPriceText,
				SellPriceText: sellPriceText,
				UpdatedAt:     updatedAt,
			})
		})

		if err = c.stg.SaveGoldPrices(goldPrices, brandID, updatedAt); err != nil {
			log.Error().Err(err).Send()
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

		var golds []store.Gold

		idMatrix := map[string]string{
			"SJC":  "01",
			"N24K": "02",
			"KB":   "03",
			"TL":   "04",
			"PNJ":  "05",
			"24K":  "06",
			"999":  "07",
			"99":   "08",
			"75":   "09",
			"58.5": "10",
			"41":   "11",
		}

		codes := []string{"SJC", "N24K", "KB", "TL", "PNJ", "24K", "999", "99", "75", "58.5", "41"}
		for _, gold := range goldPriceBoard.Data {
			if !slices.Contains(codes, gold.Code) {
				continue
			}

			golds = append(golds, store.Gold{
				Code:      idMatrix[gold.Code],
				Name:      gold.Name,
				BuyPrice:  convertNumericPrice(gold.BuyPrice),
				SellPrice: convertNumericPrice(gold.SellPrice),
			})
		}

		t, _ := time.Parse("02/01/2006 15:04:05", goldPriceBoard.UpdatedAt)

		c.store.SetBoard("pnj", &store.GoldPriceBoard{
			BrandName: "PNJ",
			Golds:     golds,
			UpdatedAt: t.Format("15:04 02/01/2006"),
		})
	}
}

func (c *Crawler) crawlPNJV2() {
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
			GoldPrices []struct {
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

		brandID, err := c.stg.GetBrandID("pnj")
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		t, err := time.Parse("02/01/2006 15:04:05", goldPriceBoard.UpdatedAt)
		if err != nil {
			log.Error().Err(err).Send()
			return
		}

		updatedAt := t.Format("15:04 02/01/2006")

		var crawlMeta *storage.CrawlMeta
		crawlMeta, err = c.stg.GetCrawlMeta(brandID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		if crawlMeta.UpdatedAt == updatedAt {
			return
		}

		goldCodes := map[string]string{
			"SJC":  "pnj01",
			"N24K": "pnj02",
			"KB":   "pnj03",
			"TL":   "pnj04",
			"PNJ":  "pnj05",
			"24K":  "pnj06",
			"999":  "pnj07",
			"9920": "pnj08",
			"99":   "pnj09",
			"22K":  "pnj10",
			"75":   "pnj11",
			"68":   "pnj12",
			"65":   "pnj13",
			"61":   "pnj14",
			"58.5": "pnj15",
			"41":   "pnj16",
			"37.5": "pnj17",
			"33":   "pnj18",
		}

		goldIDs := c.stg.GetGoldIDs(brandID)
		if len(goldIDs) == 0 {
			return
		}

		var goldPrices []storage.GoldPrice

		for _, goldPrice := range goldPriceBoard.GoldPrices {
			var exists bool

			var code string
			code, exists = goldCodes[goldPrice.Code]
			if !exists {
				continue
			}

			var id int
			id, exists = goldIDs[code]
			if !exists {
				continue
			}

			buyPrice, buyPriceText := convertPriceFromNumeric(goldPrice.BuyPrice)
			if buyPrice == 0 || buyPriceText == "" {
				continue
			}
			sellPrice, sellPriceText := convertPriceFromNumeric(goldPrice.SellPrice)
			if sellPrice == 0 || sellPriceText == "" {
				continue
			}

			goldPrices = append(goldPrices, storage.GoldPrice{
				GoldID:        id,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				BuyPriceText:  buyPriceText,
				SellPriceText: sellPriceText,
				UpdatedAt:     updatedAt,
			})
		}

		if err = c.stg.SaveGoldPrices(goldPrices, brandID, updatedAt); err != nil {
			log.Error().Err(err).Send()
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

		var golds []store.Gold

		idMatrix := map[string]string{
			"Vàng miếng VRTL Bảo Tín Minh Châu":      "01",
			"Nhẫn tròn trơn Bảo Tín Minh Châu":       "02",
			"Quà mừng bản vị vàng Bảo Tín Minh Châu": "03",
			"Vàng miếng SJC":                         "04",
			"Trang sức Vàng Rồng Thăng Long 999.9":   "05",
			"Trang sức Vàng Rồng Thăng Long 99.9":    "06",
		}

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

			name = convertGoldNameBTMC(name)

			gold := store.Gold{
				Code:      idMatrix[name],
				Name:      name,
				BuyPrice:  convertStringPrice(buyPrice),
				SellPrice: convertStringPrice(sellPrice),
			}

			golds = append(golds, gold)
		})

		updatedAt := strings.TrimSpace(doc.Find("p.note span").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "Cập nhật lúc ", "")

		t, _ := time.Parse("02/01/2006 15:04", updatedAt)

		c.store.SetBoard("btmc", &store.GoldPriceBoard{
			BrandName: "Bảo Tín Minh Châu",
			Golds:     golds,
			UpdatedAt: t.Format("15:04 02/01/2006"),
		})
	}
}

func (c *Crawler) crawlBTMCV2() {
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

		var brandID int
		brandID, err = c.stg.GetBrandID("btmc")
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		updatedAt := strings.TrimSpace(doc.Find("p.note span").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "Cập nhật lúc ", "")

		t, _ := time.Parse("02/01/2006 15:04", updatedAt)

		updatedAt = t.Format("15:04 02/01/2006")

		var crawlMeta *storage.CrawlMeta
		crawlMeta, err = c.stg.GetCrawlMeta(brandID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		if crawlMeta.UpdatedAt == updatedAt {
			return
		}

		goldCodes := map[string]string{
			"Vàng miếng VRTL Bảo Tín Minh Châu":      "btmc01",
			"Nhẫn tròn trơn Bảo Tín Minh Châu":       "btmc02",
			"Quà mừng bản vị vàng Bảo Tín Minh Châu": "btmc03",
			"Vàng miếng SJC":                         "btmc04",
			"Trang sức Vàng Rồng Thăng Long 999.9":   "btmc05",
			"Trang sức Vàng Rồng Thăng Long 99.9":    "btmc06",
		}

		goldIDs := c.stg.GetGoldIDs(brandID)
		if len(goldIDs) == 0 {
			return
		}

		var goldPrices []storage.GoldPrice

		doc.Find("table.bd_price_home tr").Each(func(_ int, s *goquery.Selection) {
			var cells []string

			s.Find("td").Each(func(_ int, s *goquery.Selection) {
				cells = append(cells, strings.TrimSpace(s.Text()))
			})

			if len(cells) == 0 {
				return
			}

			var name = ""
			var rawBuyPrice = ""
			var rawSellPrice = ""

			pattern := regexp.MustCompile("^[0-9]+$")

			if len(cells) == 5 {
				name = strings.ReplaceAll(cells[1], "  ", " ")
				if pattern.MatchString(cells[3]) {
					rawBuyPrice = cells[3]
				}
				if pattern.MatchString(cells[4]) {
					rawSellPrice = cells[4]
				}
			} else if len(cells) == 4 {
				name = strings.ReplaceAll(cells[0], "  ", " ")
				if pattern.MatchString(cells[2]) {
					rawBuyPrice = cells[2]
				}
				if pattern.MatchString(cells[3]) {
					rawSellPrice = cells[3]
				}
			}

			if strings.Contains(strings.ToLower(name), "nguyên liệu") {
				return
			}

			name = convertGoldNameBTMC(name)

			var exists bool

			var code string
			code, exists = goldCodes[name]
			if !exists {
				return
			}

			var id int
			id, exists = goldIDs[code]
			if !exists {
				return
			}

			var buyPrice int
			var sellPrice int

			var buyPriceText string
			var sellPriceText string

			buyPrice, buyPriceText, err = convertPriceFromString(rawBuyPrice)
			if err != nil {
				return
			}
			sellPrice, sellPriceText, err = convertPriceFromString(rawSellPrice)
			if err != nil {
				return
			}

			goldPrices = append(goldPrices, storage.GoldPrice{
				GoldID:        id,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				BuyPriceText:  buyPriceText,
				SellPriceText: sellPriceText,
				UpdatedAt:     updatedAt,
			})
		})

		if err = c.stg.SaveGoldPrices(goldPrices, brandID, updatedAt); err != nil {
			log.Error().Err(err).Send()
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

		var golds []store.Gold

		idMatrix := map[string]string{
			"Nhẫn ép vỉ Kim Gia Bảo":          "01",
			"Vàng miếng SJC (Cty CP BTMH)":    "02",
			"Nhẫn ép vỉ Vàng Rồng Thăng Long": "03",
			"Đồng vàng Kim Gia Bảo hoa sen":   "04",
			"Vàng nữ trang 999.9":             "05",
			"Vàng nữ trang 99.9":              "06",
			"Tiểu Kim Cát - 0,3 chỉ":          "07",
		}

		doc.Find("table.gold-table-content tbody tr").Each(func(_ int, s *goquery.Selection) {
			var cells []string

			s.Find("td").Each(func(_ int, s *goquery.Selection) {
				cells = append(cells, strings.TrimSpace(s.Text()))
			})

			if len(cells) == 0 {
				return
			}

			gold := store.Gold{
				Code:      idMatrix[cells[0]],
				Name:      cells[0],
				BuyPrice:  cells[1],
				SellPrice: cells[2],
			}

			golds = append(golds, gold)
		})

		updatedAt := strings.TrimSpace(doc.Find("p.note").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "(Cập nhật lúc ", "")
		updatedAt = strings.ReplaceAll(updatedAt, ") (đơn vị tính: đồng/chỉ)", "")

		c.store.SetBoard("btmh", &store.GoldPriceBoard{
			BrandName: "Bảo Tín Mạnh Hải",
			Golds:     golds,
			UpdatedAt: updatedAt,
		})
	}
}

func (c *Crawler) crawlBTMHV2() {
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

		goldCodes := map[string]string{
			"Nhẫn ép vỉ Kim Gia Bảo":          "btmh01",
			"Vàng miếng SJC (Cty CP BTMH)":    "btmh02",
			"Nhẫn ép vỉ Vàng Rồng Thăng Long": "btmh03",
			"Đồng vàng Kim Gia Bảo hoa sen":   "btmh04",
			"Vàng nữ trang 999.9":             "btmh05",
			"Vàng nữ trang 99.9":              "btmh06",
			"Tiểu Kim Cát - 0,3 chỉ":          "btmh07",
		}

		var brandID int
		brandID, err = c.stg.GetBrandID("btmh")
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		updatedAt := strings.TrimSpace(doc.Find("p.note").Text())
		updatedAt = strings.ReplaceAll(updatedAt, "(Cập nhật lúc ", "")
		updatedAt = strings.ReplaceAll(updatedAt, ") (đơn vị tính: đồng/chỉ)", "")

		var crawlMeta *storage.CrawlMeta
		crawlMeta, err = c.stg.GetCrawlMeta(brandID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Error().Err(err).Send()
			}
			return
		}

		if crawlMeta.UpdatedAt == updatedAt {
			return
		}

		goldIDs := c.stg.GetGoldIDs(brandID)
		if len(goldIDs) == 0 {
			return
		}

		var goldPrices []storage.GoldPrice

		doc.Find("table.gold-table-content tbody tr").Each(func(_ int, s *goquery.Selection) {
			var cells []string

			s.Find("td").Each(func(_ int, s *goquery.Selection) {
				cells = append(cells, strings.TrimSpace(s.Text()))
			})

			if len(cells) == 0 {
				return
			}

			name := cells[0]

			var exists bool

			var code string
			code, exists = goldCodes[name]
			if !exists {
				return
			}

			var id int
			id, exists = goldIDs[code]
			if !exists {
				return
			}

			var buyPrice int
			var sellPrice int

			var buyPriceText string
			var sellPriceText string

			buyPrice, buyPriceText, err = convertPriceFromString(cells[1])
			if err != nil {
				return
			}
			sellPrice, sellPriceText, err = convertPriceFromString(cells[2])
			if err != nil {
				return
			}

			goldPrices = append(goldPrices, storage.GoldPrice{
				GoldID:        id,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				BuyPriceText:  buyPriceText,
				SellPriceText: sellPriceText,
				UpdatedAt:     updatedAt,
			})
		})

		if err = c.stg.SaveGoldPrices(goldPrices, brandID, updatedAt); err != nil {
			log.Error().Err(err).Send()
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
	return strings.Join(result, ",")
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
	return strings.Join(result, ",")
}

func convertPriceFromString(s string) (int, string, error) {
	if s == "" {
		return 0, "", nil
	}

	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, ".", "")

	n, err := strconv.Atoi(s)
	if err != nil {
		log.Error().Err(err).Send()
		return 0, "", err
	}
	n = n * 1000

	s = strconv.Itoa(n)
	var result []string

	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}

	result = append([]string{s}, result...)
	return n, strings.Join(result, ","), nil
}

func convertPriceFromNumeric(n int) (int, string) {
	n = n * 1000

	s := strconv.Itoa(n)
	var result []string

	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}

	result = append([]string{s}, result...)
	return n, strings.Join(result, ",")
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
