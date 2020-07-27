package vimeo

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/akiomik/vimeo-dl/config"
)

type Client struct {
	Client    *http.Client
	UserAgent string
}

func NewClient() *Client {
	client := Client{}
	client.Client = http.DefaultClient
	client.UserAgent = "vimeo-dl/" + config.Version

	return &client
}

func (c *Client) get(url *url.URL) (*http.Response, error) {
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) GetMasterJson(url *url.URL) (*MasterJson, error) {
	res, err := c.get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	jsonBlob, err := ioutil.ReadAll(res.Body)
	masterJson := new(MasterJson)
	err = json.Unmarshal(jsonBlob, &masterJson)
	if err != nil {
		return nil, err
	}

	return masterJson, nil
}

func (c *Client) Download(url *url.URL, output io.Writer) error {
	res, err := c.get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = io.Copy(output, res.Body)
	if err != nil {
		return err
	}

	return nil
}
