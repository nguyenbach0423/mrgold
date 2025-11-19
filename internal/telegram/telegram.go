package telegram

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mrgold/internal/crawler"
	"github.com/mrgold/internal/httpclient"
	"github.com/rs/zerolog/log"
)

type Client struct {
	httpClient *httpclient.Client
	baseURL    string
	crawler    *crawler.Crawler
}

func NewClient(opts ...func(*Client)) *Client {
	c := &Client{
		httpClient: httpclient.NewClient(),
		crawler:    crawler.NewCrawler(),
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

func WithCrawler(crawler *crawler.Crawler) func(*Client) {
	return func(c *Client) {
		c.crawler = crawler
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
	reqBody, err := json.Marshal(loadMenu())
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
		reqBody, err = json.Marshal(loadGoldBranchOptions(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if command == "/gold_alert" || command == "/gold_history" || command == "/feedback" || command == "/donate" {
		reqBody, err = json.Marshal(loadComingSoon(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else {
		reqBody, err = json.Marshal(loadIntro(extras))
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
	switch command {
	case "/next_price_board_sjc", "/next_price_board_doji", "/next_price_board_pnj", "/next_price_board_btmc", "/next_price_board_btmh":
		brand := strings.ReplaceAll(command, "/next_price_board_", "")
		board := c.crawler.Boards[brand]

		reqBody, err = json.Marshal(loadGoldPriceBoard(board, extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	case "/back_branch_options":
		reqBody, err = json.Marshal(loadGoldBranchOptions(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	default:
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

func loadMenu() map[string]interface{} {
	return map[string]interface{}{
		"commands": []map[string]string{
			{"command": "gold_live", "description": "tra cứu giá vàng mới nhất"},
			{"command": "gold_alert", "description": "cảnh báo biến động giá vàng"},
			{"command": "gold_history", "description": "tra cứu lịch sử giá vàng"},
			{"command": "feedback", "description": "gửi góp ý cải thiện bot"},
			{"command": "donate", "description": "☕︎ give me a coffee cup"},
		},
	}
}

func loadIntro(extras map[string]interface{}) map[string]interface{} {
	builder := strings.Builder{}

	builder.WriteString("╭━┳━╭━╭━╮╮\n")
	builder.WriteString("┃┈┈┈┣▅╋▅┫┃\n")
	builder.WriteString("┃┈┃┈╰━╰━━━━━━╮\n")
	builder.WriteString("╰┳╯┈┈┈┈┈┈┈┈┈◢▉◣\n")
	builder.WriteString("╲┃┈┈┈┈┈┈┈┈┈┈▉▉▉\n")
	builder.WriteString("╲┃┈┈┈┈┈┈┈┈┈┈◥▉◤\n")
	builder.WriteString("╲┃┈┈┈┈╭━┳━━━━╯\n")
	builder.WriteString("╲┣━━━━━━┫\n")
	builder.WriteString("\n")
	builder.WriteString("<b>Tra cứu và cảnh báo giá vàng</b>\n")
	builder.WriteString("<code><b>/gold_live</b></code> - <i>tra cứu giá vàng mới nhất</i>\n")
	builder.WriteString("<code><b>/gold_alert</b></code> - <i>cảnh báo biến động giá vàng</i>\n")
	builder.WriteString("<code><b>/gold_history</b></code> - <i>tra cứu lịch sử giá vàng</i>\n")
	builder.WriteString("\n")
	builder.WriteString("<b>Góp ý và ủng hộ</b>\n")
	builder.WriteString("<code><b>/feedback</b></code> - <i>gửi góp ý cải thiện bot</i>\n")
	builder.WriteString("<code><b>/donate</b></code> - <i>☕︎ give me a coffee cup</i>\n")
	builder.WriteString("\n")
	builder.WriteString("<b>Hãy ra lệnh cho tôi!</b>\n")

	intro := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       builder.String(),
	}

	for k, v := range extras {
		intro[k] = v
	}

	return intro
}

func loadGoldBranchOptions(extras map[string]interface{}) map[string]interface{} {
	goldBranchOptions := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       "<b>Vui lòng chọn thương hiệu trong danh sách sau:</b>",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": []interface{}{
				[]interface{}{
					map[string]interface{}{
						"text":          "SJC",
						"callback_data": "/next_price_board_sjc",
					},
					map[string]interface{}{
						"text":          "DOJI",
						"callback_data": "/next_price_board_doji",
					},
					map[string]interface{}{
						"text":          "PNJ",
						"callback_data": "/next_price_board_pnj",
					},
				},
				[]interface{}{
					map[string]interface{}{
						"text":          "Bảo Tín Minh Châu",
						"callback_data": "/next_price_board_btmc",
					},
					map[string]interface{}{
						"text":          "Bảo Tín Mạnh Hải",
						"callback_data": "/next_price_board_btmh",
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

func loadGoldPriceBoard(board *crawler.GoldPriceBoard, extras map[string]interface{}) map[string]interface{} {
	builder := strings.Builder{}

	if board == nil || len(board.Golds) == 0 {
		builder.WriteString("<b>Giá vàng đang được cập nhật. Vui lòng thử lại trong giây lát!</b>")
	} else {
		builder.WriteString(fmt.Sprintf("<b>Bảng giá vàng tại %s:</b>\n", board.Brand))
		builder.WriteString("\n")
		for _, gold := range board.Golds {
			builder.WriteString(fmt.Sprintf("✦ <b>%s</b> - <i>Mua:</i> <b>%s</b> - <i>Bán:</i> <b>%s</b>\n", gold.Name, gold.BuyPrice, gold.SellPrice))
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
						"callback_data": "/back_branch_options",
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

func loadComingSoon(extras map[string]interface{}) map[string]interface{} {
	comingSoon := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       "<b>Tính năng sẽ sớm được ra mắt!</b>",
	}

	for k, v := range extras {
		comingSoon[k] = v
	}

	return comingSoon
}
