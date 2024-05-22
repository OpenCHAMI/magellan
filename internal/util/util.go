package util

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func GetNextIP(ip *net.IP, inc uint) *net.IP {
	if ip == nil {
		return &net.IP{}
	}
	i := ip.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += inc
	v3 := byte(v & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v0 := byte((v >> 24) & 0xFF)
	// return &net.IP{[]byte{v0, v1, v2, v3}}
	r := net.IPv4(v0, v1, v2, v3)
	return &r
}

// Generic convenience function used to make HTTP requests.
func MakeRequest(client *http.Client, url string, httpMethod string, body []byte, headers map[string]string) (*http.Response, []byte, error) {
	// use defaults if no client provided
	if client == nil {
		client = http.DefaultClient
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	req, err := http.NewRequest(httpMethod, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new HTTP request: %v", err)
	}
	req.Header.Add("User-Agent", "magellan")
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make request: %v", err)
	}
	b, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %v", err)
	}
	return res, b, err
}

func MakeOutputDirectory(path string) (string, error) {
	// get the current data + time using Go's stupid formatting
	t := time.Now()
	dirname := t.Format("2006-01-01 15:04:05")
	final := path + "/" + dirname

	// check if path is valid and directory
	pathExists, err := PathExists(final)
	if err != nil {
		return final, fmt.Errorf("failed to check for existing path: %v", err)
	}
	if pathExists {
		// make sure it is directory with 0o644 permissions
		return final, fmt.Errorf("found existing path: %v", final)
	}

	// create directory with data + time
	err = os.MkdirAll(final, 0766)
	if err != nil {
		return final, fmt.Errorf("failed to make directory: %v", err)
	}
	return final, nil
}

func SplitPathForViper(path string) (string, string, string) {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return filepath.Dir(path), strings.TrimSuffix(filename, ext), strings.TrimPrefix(ext, ".")
}

func FormatErrorList(errList []error) error {
	var err error
	for i, e := range errList {
		err = fmt.Errorf("\t[%d] %v\n", i, e)
		i += 1
	}
	return err
}

func HasErrors(errList []error) bool {
	return len(errList) > 0
}
