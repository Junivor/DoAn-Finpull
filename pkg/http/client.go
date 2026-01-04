package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	MethodGet    = http.MethodGet
	MethodPost   = http.MethodPost
	MethodPut    = http.MethodPut
	MethodDelete = http.MethodDelete
	MethodPatch  = http.MethodPatch
)

// ClientOption configures HTTPClient.
type ClientOption func(*Client)

// RequestOptions holds HTTP request parameters.
type RequestOptions struct {
	Method      string
	URL         string
	Headers     map[string]string
	QueryParams map[string][]string
	Body        interface{}
}

// Client represents an HTTP client with configurable timeout.
type Client struct {
	timeout time.Duration
	client  *http.Client
}

// NewClient creates a new HTTP client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.client = &http.Client{Timeout: c.timeout}
	return c
}

// SendRequest sends an HTTP request and returns response.
func (c *Client) SendRequest(ctx context.Context, opts *RequestOptions) (*http.Response, error) {
	req, err := c.buildRequest(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// SendAndParse sends request and parses JSON response.
func (c *Client) SendAndParse(ctx context.Context, opts *RequestOptions, dest interface{}) error {
	resp, err := c.SendRequest(ctx, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	if dest == nil {
		return nil
	}

	switch v := dest.(type) {
	case *[]byte:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		*v = body
	case io.Writer:
		if _, err := io.Copy(v, resp.Body); err != nil {
			return fmt.Errorf("copy body: %w", err)
		}
	default:
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("decode json: %w", err)
		}
	}

	return nil
}

func (c *Client) buildRequest(ctx context.Context, opts *RequestOptions) (*http.Request, error) {
	body, err := c.createRequestBody(opts)
	if err != nil {
		return nil, fmt.Errorf("create body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	c.addQueryParams(req, opts.QueryParams)
	c.addHeaders(req, opts.Headers)

	return req, nil
}

func (c *Client) createRequestBody(opts *RequestOptions) (io.Reader, error) {
	if opts.Body == nil {
		return nil, nil
	}

	switch v := opts.Body.(type) {
	case []byte:
		return bytes.NewBuffer(v), nil
	case *[]byte:
		return bytes.NewBuffer(*v), nil
	case io.Reader:
		return v, nil
	case string:
		return strings.NewReader(v), nil
	default:
		// Form-urlencoded
		if formData, ok := opts.Body.(map[string]string); ok {
			if ct := opts.Headers["Content-Type"]; ct == "application/x-www-form-urlencoded" {
			values := url.Values{}
				for k, v := range formData {
					values.Set(k, v)
			}
			return strings.NewReader(values.Encode()), nil
		}
		}

		// Default: JSON
		jsonBody, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, fmt.Errorf("marshal json: %w", err)
		}
		return bytes.NewBuffer(jsonBody), nil
	}
}

func (c *Client) addQueryParams(req *http.Request, params map[string][]string) {
	if len(params) > 0 {
		q := req.URL.Query()
		for key, values := range params {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		req.URL.RawQuery = q.Encode()
	}
}

func (c *Client) addHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if req.Header.Get("Content-Type") == "" && req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
}

// WithTimeout sets client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}
