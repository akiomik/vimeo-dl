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
    "base_url": "./bar",
    "video": [{
      "id": "1080p",
      "init_segment": "baz",
      "segments": [{
        "url": "./qux"
      }]
    }],
    "audio": null
  }`
  expected := &MasterJson{
    ClipId: "foo",
    BaseUrl: "./bar",
    Video: []Video{
      Video{
        Id: "1080p",
        InitSegment: "baz",
        Segments: []Segment{
          Segment{
            Url: "./qux",
          },
        },
      },
    },
  }

  client := NewClient()
  client.Client = NewMockClient(func (req *http.Request) *http.Response {
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
  client.Client = NewMockClient(func (req *http.Request) *http.Response {
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
