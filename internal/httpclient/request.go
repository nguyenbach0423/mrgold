package httpclient

import (
	"net/url"
)

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
