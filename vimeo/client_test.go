package vimeo

import (
	"bytes"
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
	output := new(bytes.Buffer)
	err := client.Download(parcelUrl, output)
	if err != nil {
		t.Errorf("Download request is failed: %v", err)
		return
	}

	actual := output.Bytes()
	if bytes.Compare(expected, actual) != 0 {
		t.Errorf("Download output does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}
