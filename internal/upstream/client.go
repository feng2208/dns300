package upstream

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Client struct {
	httpClientVerify   *http.Client // TLS verification enabled
	httpClientInsecure *http.Client // TLS verification disabled
}

func NewClient() *Client {
	return &Client{
		httpClientVerify: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Proxy:             http.ProxyFromEnvironment,
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: false},
				ForceAttemptHTTP2: true,
			},
		},
		httpClientInsecure: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Proxy:             http.ProxyFromEnvironment,
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
				ForceAttemptHTTP2: true,
			},
		},
	}
}

// Exchange performs a DNS query against multiple upstreams and returns the fastest valid response.
func (c *Client) Exchange(ctx context.Context, req *dns.Msg, upstreams []string, tlsVerify bool) (*dns.Msg, error) {
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("no upstreams provided")
	}

	type result struct {
		msg *dns.Msg
		err error
	}

	resultChan := make(chan result, len(upstreams))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	for _, u := range upstreams {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			// Select query method based on upstream format
			var msg *dns.Msg
			var err error

			if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
				msg, err = c.queryDoH(ctx, req, u, tlsVerify)
			} else {
				// Assume UDP regular DNS if not http(s)
				if !strings.Contains(u, ":") {
					u = u + ":53"
				}
				msg, err = c.queryUDP(ctx, req, u)
			}

			select {
			case resultChan <- result{msg: msg, err: err}:
			case <-ctx.Done():
			}
		}(u)
	}

	// Helper to wait for all goroutines to finish if we don't get a success result early
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var lastErr error
	for {
		select {
		case res, ok := <-resultChan:
			if !ok {
				// Channel closed, all tried
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, fmt.Errorf("all upstreams failed")
			}

			if res.err == nil && res.msg != nil {
				return res.msg, nil
			}
			if res.err != nil {
				lastErr = res.err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) queryUDP(ctx context.Context, req *dns.Msg, addr string) (*dns.Msg, error) {
	client := new(dns.Client)
	// Inherit timeout if context has deadline, otherwise default
	if deadline, ok := ctx.Deadline(); ok {
		client.Timeout = time.Until(deadline)
	} else {
		client.Timeout = 5 * time.Second
	}

	msg, _, err := client.ExchangeContext(ctx, req, addr)
	return msg, err
}

func (c *Client) queryDoH(ctx context.Context, req *dns.Msg, urlStr string, tlsVerify bool) (*dns.Msg, error) {
	packed, err := req.Pack()
	if err != nil {
		return nil, err
	}

	// DoH POST request with DNS message in body
	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(packed))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/dns-message")
	httpReq.Header.Set("Accept", "application/dns-message")

	// Select pre-initialized client based on TLS verification setting
	client := c.httpClientVerify
	if !tlsVerify {
		client = c.httpClientInsecure
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH server returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	msg := new(dns.Msg)
	if err := msg.Unpack(body); err != nil {
		return nil, err
	}

	return msg, nil
}
