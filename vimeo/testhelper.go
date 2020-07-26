package vimeo

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type MockRoundTripper func(req *http.Request) *http.Response

func (mock MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return mock(req), nil
}

func NewMockClient(mock MockRoundTripper) *http.Client {
	return &http.Client{
		Transport: mock,
	}
}

func NewMockReponseFromString(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
	}
}

func NewMockReponseFromBytes(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBuffer(body)),
	}
}
