package upstream

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false}, // Default to strict
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

	// DoH GET request with base64url encoded query
	b64 := base64.RawURLEncoding.EncodeToString(packed)
	reqUrl := fmt.Sprintf("%s?dns=%s", urlStr, b64)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/dns-message")

	// Create custom client if tlsVerify needs to be changed (optimized to reuse if possible in future)
	// For now, if verification is disabled, we clone the transport and disable it.
	client := c.httpClient
	if !tlsVerify {
		// Clone transport to safely modify TLS config
		transport := client.Transport.(*http.Transport).Clone()
		transport.TLSClientConfig.InsecureSkipVerify = true
		client = &http.Client{
			Timeout:   client.Timeout,
			Transport: transport,
		}
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
