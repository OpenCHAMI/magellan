package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)


func MakeRequest(url string, httpMethod string, body []byte) (*http.Response, []byte, error) {
	// url := getSmdEndpointUrl(endpoint)
	req, _ := http.NewRequest(httpMethod, url, bytes.NewBuffer(body))
	req.Header.Add("User-Agent", "magellan")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("could not make request: %v", err)
	}
	b, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("could not read response body: %v", err)
	}
	return res, b, err
}