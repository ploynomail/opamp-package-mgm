package opamppackagemgm

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
)

// Requester interface allows developers to customize the method in which
// requests are made to retrieve the version and binary.
type Requester interface {
	Fetch(url string) (io.ReadCloser, error)
	SetHeader(header map[string]string)
}

// HTTPRequester is the normal requester that is used and does an HTTP
// to the URL location requested to retrieve the specified data.
type HTTPRequester struct {
	Hearder map[string]string
}

// Fetch will return an HTTP request to the specified url and return
// the body of the result. An error will occur for a non 200 status code.
func (httpRequester *HTTPRequester) Fetch(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if httpRequester.Hearder != nil {
		for key, value := range httpRequester.Hearder {
			req.Header.Add(key, value)
		}
	}
	var client *http.Client
	// 忽略https证书
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client = &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad http status from %s: %v", url, resp.Status)
	}
	return resp.Body, nil
}

func (httpRequester *HTTPRequester) SetHeader(header map[string]string) {
	for key, value := range header {
		httpRequester.Hearder = make(map[string]string)
		httpRequester.Hearder[key] = value
	}
}

func NewHTTPRequester() *HTTPRequester {
	return &HTTPRequester{}
}
