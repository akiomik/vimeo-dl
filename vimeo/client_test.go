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
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestGetMasterJson(t *testing.T) {
	body := `{
    "clip_id": "foo",
    "base_url": "../",
    "video": [{
      "id": "bar",
      "base_url": "bar/chop/",
      "bitrate": 574000,
      "init_segment": "baz",
      "segments": [{
        "url": "segment-1.m4s"
      }]
    }],
    "audio": [{
      "id": "bar",
      "base_url": "../audio/bar/chop/",
      "bitrate": 255000,
      "init_segment": "baz",
      "segments": [{
        "url": "segment-1.m4s"
      }]
    }]
  }`
	expected := &MasterJson{
		ClipId:  "foo",
		BaseUrl: "../",
		Video: []Video{
			Video{
				Id:          "bar",
				BaseUrl:     "bar/chop/",
				Bitrate:     574000,
				InitSegment: "baz",
				Segments: []Segment{
					Segment{
						Url: "segment-1.m4s",
					},
				},
			},
		},
		Audio: []Audio{
			Audio{
				Id:          "bar",
				BaseUrl:     "../audio/bar/chop/",
				Bitrate:     255000,
				InitSegment: "baz",
				Segments: []Segment{
					Segment{
						Url: "segment-1.m4s",
					},
				},
			},
		},
	}

	client := NewClient()
	client.Client = NewMockClient(func(req *http.Request) *http.Response {
		return NewMockReponseFromString(body)
	})

	jsonUrl, _ := url.Parse("http://example.com/master.json")
	actual, err := client.GetMasterJson(jsonUrl)
	if err != nil {
		t.Errorf("GetMasterJson request is failed: %v", err)
		return
	}

	if !reflect.DeepEqual(*expected, *actual) {
		t.Errorf("GetMasterJson response does not match.\nexpected: %v\nactual:   %v", *expected, *actual)
		return
	}
}

func TestDownload(t *testing.T) {
	body := "0123456789"
	expected := []byte(body)

	client := NewClient()
	client.Client = NewMockClient(func(req *http.Request) *http.Response {
		return NewMockReponseFromString(body)
	})

	parcelUrl, _ := url.Parse("http://example.com/parcel/1080.mp4?range=0-100")
	file, err := client.Download(parcelUrl)
	defer Cleanup(file)
	if err != nil {
		t.Errorf("Download request is failed: %v", err)
		return
	}

	v, err := ioutil.ReadFile(file.Name())
	if !bytes.Equal(expected, v) {
		t.Errorf("Download output does not match.\nexpected: %v\nv:   %v", expected, v)
		return
	}
}
