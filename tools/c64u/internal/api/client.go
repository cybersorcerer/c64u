package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client represents an HTTP client for the C64 Ultimate REST API
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Verbose    bool
}

// Response represents a standard API response
type Response struct {
	Errors     []string               `json:"errors"`
	Data       map[string]interface{} `json:",inline"`
	StatusCode int                    `json:"-"`
	RawBody    []byte                 `json:"-"`
}

// NewClient creates a new API client
func NewClient(host string, port int, verbose bool) *Client {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)

	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Verbose: verbose,
	}
}

// Get performs a GET request to the API
func (c *Client) Get(endpoint string, params map[string]string) (*Response, error) {
	// Build URL with query parameters
	reqURL, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if len(params) > 0 {
		query := reqURL.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		reqURL.RawQuery = query.Encode()
	}

	if c.Verbose {
		fmt.Printf("→ GET %s\n", reqURL.String())
	}

	resp, err := c.HTTPClient.Get(reqURL.String())
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

// Put performs a PUT request to the API
func (c *Client) Put(endpoint string, params map[string]string) (*Response, error) {
	// Build URL with query parameters
	reqURL, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if len(params) > 0 {
		query := reqURL.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		reqURL.RawQuery = query.Encode()
	}

	if c.Verbose {
		fmt.Printf("→ PUT %s\n", reqURL.String())
	}

	req, err := http.NewRequest(http.MethodPut, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

// Post performs a POST request to the API with a body
func (c *Client) Post(endpoint string, body io.Reader, params map[string]string) (*Response, error) {
	// Build URL with query parameters
	reqURL, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if len(params) > 0 {
		query := reqURL.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		reqURL.RawQuery = query.Encode()
	}

	if c.Verbose {
		fmt.Printf("→ POST %s\n", reqURL.String())
	}

	req, err := http.NewRequest(http.MethodPost, reqURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set appropriate content type
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

// PostJSON performs a POST request with JSON body
func (c *Client) PostJSON(endpoint string, data interface{}) (*Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	reqURL := c.BaseURL + endpoint

	if c.Verbose {
		fmt.Printf("→ POST %s\n", reqURL)
		fmt.Printf("  Body: %s\n", string(jsonData))
	}

	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

// parseResponse parses the HTTP response and extracts error information
func (c *Client) parseResponse(resp *http.Response) (*Response, error) {
	// Read the entire response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if c.Verbose {
		fmt.Printf("← %d %s\n", resp.StatusCode, resp.Status)
		if len(body) > 0 {
			fmt.Printf("  Response: %s\n", string(body))
		}
	}

	apiResp := &Response{
		StatusCode: resp.StatusCode,
		RawBody:    body,
		Data:       make(map[string]interface{}),
	}

	// Try to parse as JSON
	if len(body) > 0 {
		// First try to unmarshal into a generic map to get all fields
		var jsonData map[string]interface{}
		if err := json.Unmarshal(body, &jsonData); err != nil {
			// Not JSON or invalid JSON - store as raw body
			return apiResp, nil
		}

		// Extract errors array if present
		if errors, ok := jsonData["errors"].([]interface{}); ok {
			for _, e := range errors {
				if errStr, ok := e.(string); ok {
					apiResp.Errors = append(apiResp.Errors, errStr)
				}
			}
			delete(jsonData, "errors")
		}

		// Store remaining data
		apiResp.Data = jsonData
	}

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(apiResp.Errors) == 0 {
			apiResp.Errors = append(apiResp.Errors, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status))
		}
	}

	return apiResp, nil
}

// HasErrors returns true if the response contains errors
func (r *Response) HasErrors() bool {
	return len(r.Errors) > 0
}

// GetString safely retrieves a string value from the response data
func (r *Response) GetString(key string) string {
	if val, ok := r.Data[key].(string); ok {
		return val
	}
	return ""
}

// GetInt safely retrieves an int value from the response data
func (r *Response) GetInt(key string) int {
	if val, ok := r.Data[key].(float64); ok {
		return int(val)
	}
	return 0
}
