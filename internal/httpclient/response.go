package httpclient

import "encoding/json"

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
