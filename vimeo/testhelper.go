// Copyright 2020 Akiomi Kamakura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vimeo

import (
	"bytes"
	"io"
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
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func NewMockReponseFromBytes(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(body)),
	}
}
