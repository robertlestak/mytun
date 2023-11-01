package request

import (
	"bytes"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Define a struct to represent the request data
type RequestData struct {
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

type ResponseData struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (r *RequestData) SendLocal(address string) (*ResponseData, error) {
	l := log.WithFields(log.Fields{
		"app":           "mytun",
		"cmd":           "client.SendLocal",
		"method":        r.Method,
		"url":           r.URL,
		"local-address": address,
	})
	l.Debug("Sending request to local server")
	req, err := http.NewRequest(r.Method, address+r.URL, bytes.NewReader(r.Body))
	if err != nil {
		return nil, err
	}
	req.Header = r.Header
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return &ResponseData{
		StatusCode: res.StatusCode,
		Header:     res.Header,
		Body:       body,
	}, nil
}
