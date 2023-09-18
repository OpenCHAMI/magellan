package util

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func PathExists(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil { return true, nil }
    if os.IsNotExist(err) { return false, nil }
    return false, err
}

func MakeRequest(url string, httpMethod string, body []byte, headers map[string]string) (*http.Response, []byte, error) {
	// url := getSmdEndpointUrl(endpoint)
	req, _ := http.NewRequest(httpMethod, url, bytes.NewBuffer(body))
	req.Header.Add("User-Agent", "magellan")
	for k, v := range headers {
		req.Header.Add(k, v)
	}
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

func MakeOutputDirectory(path string) (string, error) {
    // get the current data + time using Go's stupid formatting
    t := time.Now()
    dirname := t.Format("2006-01-01 15:04:05")
    final := path + "/" + dirname

	// check if path is valid and directory
    pathExists, err := PathExists(final); 
    if err != nil {
        return final, fmt.Errorf("could not check for existing path: %v", err) 
    }
	if pathExists {
        // make sure it is directory with 0o644 permissions
        return final, fmt.Errorf("found existing path: %v", final)
    }
	
	// create directory with data + time
	err = os.MkdirAll(final, 0766)
	if err != nil {
		return final, fmt.Errorf("could not make directory: %v", err)
	}
	return final, nil
}