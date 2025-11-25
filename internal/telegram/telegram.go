package telegram

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mrgold/internal/httpclient"
	"github.com/mrgold/internal/store"
	"github.com/rs/zerolog/log"
)

type Client struct {
	httpClient *httpclient.Client
	baseURL    string
	store      *store.Store
}

func NewClient(opts ...func(*Client)) *Client {
	c := &Client{
		httpClient: httpclient.NewClient(),
		store:      store.NewStore(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithHTTPClient(httpClient *httpclient.Client) func(*Client) {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithBaseURL(baseURL string) func(*Client) {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func WithStore(store *store.Store) func(*Client) {
	return func(c *Client) {
		c.store = store
	}
}

type Update struct {
	UpdateId      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageId int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      *Chat  `json:"chat"`
	Text      string `json:"text"`
}

type User struct {
	Id int `json:"id"`
}

type Chat struct {
	Id int `json:"id"`
}

type CallbackQuery struct {
	Id      string   `json:"id"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

func (c *Client) SetWebhook(publicDomain string) bool {
	reqBody, err := json.Marshal(map[string]string{
		"url": publicDomain + "/webhook",
	})
	if err != nil {
		log.Error().Err(err).Send()
		return false
	}

	_, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodPost,
			c.baseURL+"/setWebhook",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
			httpclient.WithBody(reqBody),
		),
	)

	return ok
}

func (c *Client) SetMyCommands() bool {
	reqBody, err := json.Marshal(c.loadMenu())
	if err != nil {
		log.Error().Err(err).Send()
		return false
	}

	_, ok := c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodPost,
			c.baseURL+"/setMyCommands",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
			httpclient.WithBody(reqBody),
		),
	)

	return ok
}

func (c *Client) HandleMessage(message *Message) {
	go c.sendMessage(message.Text, map[string]interface{}{
		"chat_id": message.Chat.Id,
	})
}

func (c *Client) sendMessage(command string, extras map[string]interface{}) {
	var err error

	var reqBody []byte
	if command == "/gold_live" {
		reqBody, err = json.Marshal(c.loadLiveGoldBranchOptions(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if command == "/gold_history" {
		reqBody, err = json.Marshal(c.loadHistoricalGoldBranchOptions(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if command == "/gold_alert" || command == "/feedback" || command == "/donate" {
		reqBody, err = json.Marshal(c.loadComingSoon(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else {
		reqBody, err = json.Marshal(c.loadIntro(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	}

	c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodPost,
			c.baseURL+"/sendMessage",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
			httpclient.WithBody(reqBody),
		),
	)
}

func (c *Client) HandleCallbackQuery(callbackQuery *CallbackQuery) {
	go c.answerCallbackQuery(callbackQuery.Id)

	go c.editMessageText(callbackQuery.Data, map[string]interface{}{
		"chat_id":    callbackQuery.Message.Chat.Id,
		"message_id": callbackQuery.Message.MessageId,
	})
}

func (c *Client) answerCallbackQuery(id string) {
	reqBody, err := json.Marshal(map[string]string{
		"callback_query_id": id,
	})
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodPost,
			c.baseURL+"/answerCallbackQuery",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
			httpclient.WithBody(reqBody),
		),
	)
}

func (c *Client) editMessageText(command string, extras map[string]interface{}) {
	var err error

	var reqBody []byte
	if slices.Contains([]string{"/next_live_price_board_sjc", "/next_live_price_board_doji", "/next_live_price_board_pnj", "/next_live_price_board_btmc", "/next_live_price_board_btmh"}, command) {
		brand := strings.ReplaceAll(command, "/next_live_price_board_", "")
		board := c.store.GetBoard(brand)

		reqBody, err = json.Marshal(c.loadGoldPriceBoard(board, extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if command == "/back_live_branch_options" {
		reqBody, err = json.Marshal(c.loadLiveGoldBranchOptions(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if slices.Contains([]string{"/next_historical_product_options_sjc", "/next_historical_product_options_doji", "/next_historical_product_options_pnj", "/next_historical_product_options_btmc", "/next_historical_product_options_btmh"}, command) {
		brand := strings.ReplaceAll(command, "/next_historical_product_options_", "")

		reqBody, err = json.Marshal(c.loadHistoricalProductOptions(brand, extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if command == "/back_historical_branch_options" {
		reqBody, err = json.Marshal(c.loadHistoricalGoldBranchOptions(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if strings.HasPrefix(command, "/next_historical_time_range_") {
		metaInfo := strings.ReplaceAll(command, "/next_historical_time_range_", "")

		parts := strings.Split(metaInfo, "_")

		brand := parts[0]
		product := parts[1]

		reqBody, err = json.Marshal(c.loadHistoricalTimeRangeOptions(brand, product, extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if strings.HasPrefix(command, "/back_historical_product_options_") {
		metaInfo := strings.ReplaceAll(command, "/back_historical_product_options_", "")

		brand := strings.Split(metaInfo, "_")[0]

		reqBody, err = json.Marshal(c.loadHistoricalProductOptions(brand, extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if strings.HasPrefix(command, "/next_historical_gold_price_") {
		metaInfo := strings.ReplaceAll(command, "/next_historical_gold_price_", "")

		parts := strings.Split(metaInfo, "_")

		var timeRange int
		timeRange, err = strconv.Atoi(strings.TrimSuffix(parts[0], "d"))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
		brand := parts[1]
		product := parts[2]

		reqBody, err = json.Marshal(c.loadHistoricalGoldPrice(brand, product, timeRange, extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if strings.HasPrefix(command, "/back_historical_time_range_options_") {
		metaInfo := strings.ReplaceAll(command, "/back_historical_time_range_options_", "")

		parts := strings.Split(metaInfo, "_")
		brand := parts[0]
		product := parts[1]

		reqBody, err = json.Marshal(c.loadHistoricalTimeRangeOptions(brand, product, extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else {
		return
	}

	c.httpClient.Do(
		httpclient.NewRequest(
			http.MethodPost,
			c.baseURL+"/editMessageText",
			httpclient.WithHeaders(httpclient.DefaultHeaders),
			httpclient.WithBody(reqBody),
		),
	)
}

func (c *Client) loadMenu() map[string]interface{} {
	return map[string]interface{}{
		"commands": []map[string]string{
			{"command": "gold_live", "description": "Tra cứu giá vàng mới nhất"},
			{"command": "gold_alert", "description": "Cảnh báo biến động giá vàng"},
			{"command": "gold_history", "description": "Tra cứu lịch sử giá vàng"},
			{"command": "feedback", "description": "Gửi góp ý cải thiện bot"},
			{"command": "donate", "description": "☕︎ Buy me a coffee"},
		},
	}
}

func (c *Client) loadIntro(extras map[string]interface{}) map[string]interface{} {
	builder := strings.Builder{}

	builder.WriteString("<b>Tra cứu và cảnh báo giá vàng</b>\n")
	builder.WriteString("\n")
	builder.WriteString("<code><b>/gold_live</b></code> - <i>Tra cứu giá vàng mới nhất</i>\n")
	builder.WriteString("<code><b>/gold_alert</b></code> - <i>Cảnh báo biến động giá vàng</i>\n")
	builder.WriteString("<code><b>/gold_history</b></code> - <i>Tra cứu lịch sử giá vàng</i>\n")
	builder.WriteString("\n")
	builder.WriteString("<b>Góp ý và ủng hộ</b>\n")
	builder.WriteString("\n")
	builder.WriteString("<code><b>/feedback</b></code> - <i>Gửi góp ý cải thiện bot</i>\n")
	builder.WriteString("<code><b>/donate</b></code> - <i>☕︎ Buy me a coffee</i>\n")

	intro := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       builder.String(),
	}

	for k, v := range extras {
		intro[k] = v
	}

	return intro
}

func (c *Client) loadLiveGoldBranchOptions(extras map[string]interface{}) map[string]interface{} {
	goldBranchOptions := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       "<b>Vui lòng chọn thương hiệu trong danh sách sau:</b>",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": []interface{}{
				[]interface{}{
					map[string]interface{}{
						"text":          "SJC",
						"callback_data": "/next_live_price_board_sjc",
					},
					map[string]interface{}{
						"text":          "DOJI",
						"callback_data": "/next_live_price_board_doji",
					},
					map[string]interface{}{
						"text":          "PNJ",
						"callback_data": "/next_live_price_board_pnj",
					},
				},
				[]interface{}{
					map[string]interface{}{
						"text":          "Bảo Tín Minh Châu",
						"callback_data": "/next_live_price_board_btmc",
					},
					map[string]interface{}{
						"text":          "Bảo Tín Mạnh Hải",
						"callback_data": "/next_live_price_board_btmh",
					},
				},
			},
		},
	}

	for k, v := range extras {
		goldBranchOptions[k] = v
	}

	return goldBranchOptions
}

func (c *Client) loadGoldPriceBoard(board *store.GoldPriceBoard, extras map[string]interface{}) map[string]interface{} {
	builder := strings.Builder{}

	if board == nil || len(board.Golds) == 0 {
		builder.WriteString("<b>Giá vàng đang được cập nhật. Vui lòng thử lại trong giây lát!</b>")
	} else {
		builder.WriteString(fmt.Sprintf("<b>Bảng giá vàng tại %s:</b>\n", board.BrandName))
		builder.WriteString("\n")
		for _, gold := range board.Golds {
			builder.WriteString(fmt.Sprintf("✦ <b>%s</b>", gold.Name))
			if gold.BuyPrice != "" {
				builder.WriteString(fmt.Sprintf(" - <i>Mua:</i> <b>%s</b>", gold.BuyPrice))
			}
			if gold.SellPrice != "" {
				builder.WriteString(fmt.Sprintf(" - <i>Bán:</i> <b>%s</b>", gold.SellPrice))
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("<i>(Cập nhật lúc: %s</i> - <i>Đơn vị tính: đồng/chỉ)</i>", board.UpdatedAt))
	}

	goldPriceBoard := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       builder.String(),
		"reply_markup": map[string]interface{}{
			"inline_keyboard": []interface{}{
				[]interface{}{
					map[string]interface{}{
						"text":          "<< Quay lại danh sách thương hiệu",
						"callback_data": "/back_live_branch_options",
					},
				},
			},
		},
	}

	for k, v := range extras {
		goldPriceBoard[k] = v
	}

	return goldPriceBoard
}

func (c *Client) loadHistoricalGoldBranchOptions(extras map[string]interface{}) map[string]interface{} {
	goldBranchOptions := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       "<b>Bạn muốn tra cứu lịch sử của thương hiệu nào?</b>",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": []interface{}{
				[]interface{}{
					map[string]interface{}{
						"text":          "SJC",
						"callback_data": "/next_historical_product_options_sjc",
					},
					map[string]interface{}{
						"text":          "DOJI",
						"callback_data": "/next_historical_product_options_doji",
					},
					map[string]interface{}{
						"text":          "PNJ",
						"callback_data": "/next_historical_product_options_pnj",
					},
				},
				[]interface{}{
					map[string]interface{}{
						"text":          "Bảo Tín Minh Châu",
						"callback_data": "/next_historical_product_options_btmc",
					},
					map[string]interface{}{
						"text":          "Bảo Tín Mạnh Hải",
						"callback_data": "/next_historical_product_options_btmh",
					},
				},
			},
		},
	}

	for k, v := range extras {
		goldBranchOptions[k] = v
	}

	return goldBranchOptions
}

func (c *Client) loadHistoricalProductOptions(brand string, extras map[string]interface{}) map[string]interface{} {
	golds := c.store.GetBoard(brand).Golds

	keyboard := make([]interface{}, 0, len(golds))
	for _, gold := range golds {
		keyboard = append(keyboard, []interface{}{
			map[string]interface{}{
				"text":          gold.Name,
				"callback_data": fmt.Sprintf("/next_historical_time_range_%s_%s", brand, gold.Code),
			},
		})
	}

	keyboard = append(keyboard, []interface{}{
		map[string]interface{}{
			"text":          "<< Quay lại danh sách thương hiệu",
			"callback_data": "/back_historical_branch_options",
		},
	})

	productOptions := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       "<b>Bạn muốn tra cứu lịch sử của sản phẩm nào?</b>",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": keyboard,
		},
	}

	for k, v := range extras {
		productOptions[k] = v
	}

	return productOptions
}

func (c *Client) loadHistoricalTimeRangeOptions(brand string, product string, extras map[string]interface{}) map[string]interface{} {
	timeRangeOptions := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       "<b>Vui lòng chọn khoảng thời gian bạn muốn tra cứu:</b>",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": []interface{}{
				[]interface{}{
					map[string]interface{}{
						"text":          "1 ngày",
						"callback_data": fmt.Sprintf("/next_historical_gold_price_1d_%s_%s", brand, product),
					},
					map[string]interface{}{
						"text":          "3 ngày",
						"callback_data": fmt.Sprintf("/next_historical_gold_price_3d_%s_%s", brand, product),
					},
					map[string]interface{}{
						"text":          "5 ngày",
						"callback_data": fmt.Sprintf("/next_historical_gold_price_5d_%s_%s", brand, product),
					},
				},
				[]interface{}{
					map[string]interface{}{
						"text":          "<< Quay lại danh sách sản phẩm",
						"callback_data": fmt.Sprintf("/back_historical_product_options_%s_%s", brand, product),
					},
				},
			},
		},
	}

	for k, v := range extras {
		timeRangeOptions[k] = v
	}

	return timeRangeOptions
}

func (c *Client) loadHistoricalGoldPrice(brand string, product string, timeRange int, extras map[string]interface{}) map[string]interface{} {
	builder := strings.Builder{}

	now := time.Now()

	var days []string
	for i := 0; i <= timeRange; i++ {
		day := now.AddDate(0, 0, -i).Format("02/01/2006")
		days = append(days, day)
	}

	for _, day := range days {
		builder.WriteString(fmt.Sprintf("%s\n", day))
	}

	//history := c.store.GetHistory(brand)

	//if board == nil || len(board.Golds) == 0 {
	//	builder.WriteString("<b>Giá vàng đang được cập nhật. Vui lòng thử lại trong giây lát!</b>")
	//} else {
	//	builder.WriteString(fmt.Sprintf("<b>Bảng giá vàng tại %s:</b>\n", board.BrandName))
	//	builder.WriteString("\n")
	//	for _, gold := range board.Golds {
	//		builder.WriteString(fmt.Sprintf("✦ <b>%s</b>", gold.Name))
	//		if gold.BuyPrice != "" {
	//			builder.WriteString(fmt.Sprintf(" - <i>Mua:</i> <b>%s</b>", gold.BuyPrice))
	//		}
	//		if gold.SellPrice != "" {
	//			builder.WriteString(fmt.Sprintf(" - <i>Bán:</i> <b>%s</b>", gold.SellPrice))
	//		}
	//		builder.WriteString("\n")
	//	}
	//	builder.WriteString("\n")
	//	builder.WriteString(fmt.Sprintf("<i>(Cập nhật lúc: %s</i> - <i>Đơn vị tính: đồng/chỉ)</i>", board.UpdatedAt))
	//}

	goldPriceHistory := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       builder.String(),
		"reply_markup": map[string]interface{}{
			"inline_keyboard": []interface{}{
				[]interface{}{
					map[string]interface{}{
						"text":          "<< Quay lại",
						"callback_data": fmt.Sprintf("/back_historical_time_range_options_%s_%s", brand, product),
					},
				},
			},
		},
	}

	for k, v := range extras {
		goldPriceHistory[k] = v
	}

	return goldPriceHistory
}

func (c *Client) loadComingSoon(extras map[string]interface{}) map[string]interface{} {
	comingSoon := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       "<b>Tính năng sẽ sớm được ra mắt!</b>",
	}

	for k, v := range extras {
		comingSoon[k] = v
	}

	return comingSoon
}
