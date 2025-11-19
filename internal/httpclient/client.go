package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	DefaultTimeout = 5 * time.Second
)

var DefaultHeaders = map[string]string{
	"User-Agent":   "MrGoldBot/1.0 (+https://t.me/mr_gold_vn_bot)",
	"Accept":       "application/json",
	"Content-Type": "application/json",
}

type Client struct {
	httpClient  *http.Client
	retryConfig *RetryConfig
}

func NewClient(opts ...func(*Client)) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		retryConfig: &RetryConfig{
			MaxRetries: 0,
			Backoff:    0 * time.Second,
			MaxBackoff: 0 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithTimeout(timeout time.Duration) func(*Client) {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

type RetryConfig struct {
	MaxRetries int
	Backoff    time.Duration
	MaxBackoff time.Duration
}

func WithRetryConfig(retryConfig *RetryConfig) func(*Client) {
	return func(c *Client) {
		if retryConfig == nil {
			return
		}
		if retryConfig.MaxRetries < 0 {
			retryConfig.MaxRetries = 0
		}
		if retryConfig.Backoff < 0 {
			retryConfig.Backoff = 0
		}
		if retryConfig.MaxBackoff < 0 {
			retryConfig.MaxBackoff = 0
		}
		c.retryConfig = retryConfig
	}
}

func (c *Client) Do(req *Request) (*Response, bool) {
	logger := log.With().
		Str("request_id", uuid.New().String()).
		Interface("request", req.Log()).
		Logger()

	reqBody := req.Body

	var resp *Response

	maxRetries := c.retryConfig.MaxRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var reader io.Reader
		if len(req.Body) > 0 {
			reader = bytes.NewReader(reqBody)
		}

		var err error

		var httpReq *http.Request
		httpReq, err = http.NewRequest(req.Method, req.BuildURL(), reader)
		if err != nil {
			logger.Error().Err(err).Send()
			return nil, false
		}

		for k, v := range req.Headers {
			httpReq.Header.Add(k, v)
		}

		var httpResp *http.Response
		httpResp, err = c.httpClient.Do(httpReq)
		if err != nil {
			logger.Error().Err(err).Send()
			return nil, false
		}

		var respBody []byte
		respBody, err = io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		if err != nil {
			logger.Error().Err(err).Send()
			return nil, false
		}

		resp = &Response{
			StatusCode: httpResp.StatusCode,
			Body:       respBody,
		}

		if resp.StatusCode == http.StatusOK {
			return resp, true
		}

		logger.Warn().Interface("response", resp.Log()).Send()

		if resp.StatusCode >= 500 && resp.StatusCode < 599 {
			if attempt < maxRetries {
				sleepTime := c.retryConfig.Backoff * (1 << attempt)
				if sleepTime > c.retryConfig.MaxBackoff {
					sleepTime = c.retryConfig.MaxBackoff
				}

				if sleepTime > 0 {
					sleepTime = time.Duration(rand.Int63n(int64(sleepTime)))

					time.Sleep(sleepTime)
				}
			}
			continue
		}
	}

	return resp, false
}

type Request struct {
	Method      string
	URL         string
	Headers     map[string]string
	QueryParams map[string][]string
	Body        []byte
}

func NewRequest(method, url string, opts ...func(*Request)) *Request {
	r := &Request{
		Method: method,
		URL:    url,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func WithHeaders(headers map[string]string) func(*Request) {
	return func(r *Request) {
		if r.Headers == nil {
			r.Headers = make(map[string]string)
		}

		for k, v := range headers {
			r.Headers[k] = v
		}
	}
}

func WithQueryParams(queryParams map[string][]string) func(*Request) {
	return func(r *Request) {
		if r.QueryParams == nil {
			r.QueryParams = make(map[string][]string)
		}

		for k, v := range queryParams {
			r.QueryParams[k] = append(r.QueryParams[k], v...)
		}
	}
}

func WithBody(body []byte) func(*Request) {
	return func(r *Request) {
		r.Body = body
	}
}

func (r *Request) BuildURL() string {
	queryParams := r.QueryParams
	if queryParams == nil || len(queryParams) == 0 {
		return r.URL
	}

	values := url.Values{}

	for k, v := range queryParams {
		for _, i := range v {
			values.Add(k, i)
		}
	}

	return r.URL + "?" + values.Encode()
}

func (r *Request) Log() interface{} {
	body := r.Body

	var logBody string
	if len(body) > 0 {
		logBody = string(body)
	}

	return struct {
		Method      string              `json:"method"`
		URL         string              `json:"url"`
		Headers     map[string]string   `json:"headers,omitempty"`
		QueryParams map[string][]string `json:"query_params,omitempty"`
		Body        string              `json:"body,omitempty"`
	}{
		Method:      r.Method,
		URL:         r.URL,
		Headers:     r.Headers,
		QueryParams: r.QueryParams,
		Body:        logBody,
	}
}

type Response struct {
	StatusCode int
	Body       []byte
}

func (r *Response) Unmarshal(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

func (r *Response) Log() interface{} {
	body := r.Body

	var logBody string
	if len(body) > 0 {
		logBody = string(body)
	}

	return struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body,omitempty"`
	}{
		StatusCode: r.StatusCode,
		Body:       logBody,
	}
}
