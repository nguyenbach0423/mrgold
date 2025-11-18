package telegram

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mrgold/internal/httpclient"
	"github.com/rs/zerolog/log"
)

var DefaultHeaders = map[string]string{
	"User-Agent":   "MrGoldBot/1.0 (+https://t.me/mr_gold_vn_bot)",
	"Accept":       "application/json",
	"Content-Type": "application/json",
}

type Client struct {
	httpClient *httpclient.Client
	baseURL    string
}

func NewClient(opts ...func(*Client)) *Client {
	c := &Client{
		httpClient: httpclient.NewClient(),
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

func (c *Client) HandleUpdate(update *Update) {
	go c.sendMessage(update.Message.Text, map[string]interface{}{
		"chat_id": update.Message.Chat.Id,
	})
}

func (c *Client) sendMessage(command string, extras map[string]interface{}) {
	var err error

	var reqBody []byte
	if command == "/start" {
		reqBody, err = json.Marshal(loadGoldBranchOptions(extras))
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
			c.baseURL+"/sendMessage",
			httpclient.WithHeaders(DefaultHeaders),
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
			httpclient.WithHeaders(DefaultHeaders),
			httpclient.WithBody(reqBody),
		),
	)
}

func (c *Client) editMessageText(command string, extras map[string]interface{}) {
	var err error

	var reqBody []byte
	if command == "/next GoldPriceBoard BTMH" {
		reqBody, err = json.Marshal(loadGoldPriceBoard(extras))
		if err != nil {
			log.Error().Err(err).Send()
			return
		}
	} else if command == "/back GoldBranchOptions" {
		reqBody, err = json.Marshal(loadGoldBranchOptions(extras))
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
			httpclient.WithHeaders(DefaultHeaders),
			httpclient.WithBody(reqBody),
		),
	)
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
						"callback_data": "/next GoldPriceBoard SJC",
					},
					map[string]interface{}{
						"text":          "DOJI",
						"callback_data": "/next GoldPriceBoard DOJI",
					},
					map[string]interface{}{
						"text":          "PNJ",
						"callback_data": "/next GoldPriceBoard PNJ",
					},
				},
				[]interface{}{
					map[string]interface{}{
						"text":          "Bảo Tín Minh Châu",
						"callback_data": "/next GoldPriceBoard BTMC",
					},
					map[string]interface{}{
						"text":          "Bảo Tín Mạnh Hải",
						"callback_data": "/next GoldPriceBoard BTMH",
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

func loadGoldPriceBoard(extras map[string]interface{}) map[string]interface{} {
	builder := strings.Builder{}

	builder.WriteString("<b>Bảng giá vàng tại Bảo Tín Mạnh Hải:</b>\n")
	builder.WriteString("\n")
	builder.WriteString("🔶Nhẫn ép vỉ Kim Gia Bảo - Mua: 14.800.000 - Bán: 15.100.000\n")
	builder.WriteString("🔶Nhẫn ép vỉ Kim Gia Bảo - Mua: 14.800.000 - Bán: 15.100.000\n")
	builder.WriteString("🔶Nhẫn ép vỉ Kim Gia Bảo - Mua: 14.800.000 - Bán: 15.100.000\n")
	builder.WriteString("\n")
	builder.WriteString("<i>(Cập nhật lúc: 18:00:00 18/11/2025 - Đơn vị tính: đồng/chỉ)</i>")

	goldPriceBoard := map[string]interface{}{
		"parse_mode": "HTML",
		"text":       builder.String(),
		"reply_markup": map[string]interface{}{
			"inline_keyboard": []interface{}{
				[]interface{}{
					map[string]interface{}{
						"text":          "<< Quay lại danh sách thương hiệu",
						"callback_data": "/back GoldBranchOptions",
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
