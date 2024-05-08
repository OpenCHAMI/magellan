package smd

// See ref for API docs:
//	https://github.com/OpenCHAMI/hms-smd/blob/master/docs/examples.adoc
//	https://github.com/OpenCHAMI/hms-smd
import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/OpenCHAMI/magellan/internal/util"
)

var (
	Host         = "http://localhost"
	BaseEndpoint = "/hsm/v2"
	Port         = 27779
)

type Option func(*Client)

type Client struct {
	*http.Client
	CACertPool *x509.CertPool
}

func NewClient(opts ...Option) *Client {
	client := &Client{
		Client: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithHttpClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.Client = httpClient
	}
}

// This MakeRequest function is a wrapper around the util.MakeRequest function
// with a couple of niceties with using a smd.Client
func (c *Client) MakeRequest(url string, method string, body []byte, headers map[string]string) (*http.Response, []byte, error) {
	return util.MakeRequest(c.Client, url, method, body, headers)
}

func WithCertPool(certPool *x509.CertPool) Option {
	return func(c *Client) {
		c.Client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			},
			DisableKeepAlives: true,
			Dial: (&net.Dialer{
				Timeout:   120 * time.Second,
				KeepAlive: 120 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   120 * time.Second,
			ResponseHeaderTimeout: 120 * time.Second,
		}
	}
}

func WithSecureTLS(certPath string) Option {
	cacert, _ := os.ReadFile(certPath)
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cacert)
	return WithCertPool(certPool)
}

func (c *Client) GetRedfishEndpoints(headers map[string]string, opts ...Option) error {
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	_, body, err := c.MakeRequest(url, "GET", nil, headers)
	if err != nil {
		return fmt.Errorf("failed toget endpoint: %v", err)
	}
	// fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func (c *Client) GetComponentEndpoint(xname string) error {
	url := makeEndpointUrl("/Inventory/ComponentsEndpoints/" + xname)
	res, body, err := c.MakeRequest(url, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("failed toget endpoint: %v", err)
	}
	fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func (c *Client) AddRedfishEndpoint(data []byte, headers map[string]string) error {
	if data == nil {
		return fmt.Errorf("failed toadd redfish endpoint: no data found")
	}

	// Add redfish endpoint via POST `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	res, body, err := c.MakeRequest(url, "POST", data, headers)
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("returned status code %d when adding endpoint", res.StatusCode)
		}
		fmt.Printf("%v (%v)\n%s\n", url, res.Status, string(body))
	}
	return err
}

func (c *Client) UpdateRedfishEndpoint(xname string, data []byte, headers map[string]string) error {
	if data == nil {
		return fmt.Errorf("failed to add redfish endpoint: no data found")
	}
	// Update redfish endpoint via PUT `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := makeEndpointUrl("/Inventory/RedfishEndpoints/" + xname)
	res, body, err := c.MakeRequest(url, "PUT", data, headers)
	fmt.Printf("%v (%v)\n%s\n", url, res.Status, string(body))
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("failed to update redfish endpoint (returned %s)", res.Status)
		}
	}
	return err
}

func makeEndpointUrl(endpoint string) string {
	return Host + ":" + fmt.Sprint(Port) + BaseEndpoint + endpoint
}
