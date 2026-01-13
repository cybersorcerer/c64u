package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// ConfigSettings represents multiple configuration settings
type ConfigSettings map[string]map[string]interface{}

// GetConfigCategories retrieves all configuration categories
func (c *Client) GetConfigCategories() ([]string, error) {
	resp, err := c.Get("/v1/configs", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(resp.RawBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse categories: %w", err)
	}

	return result.Categories, nil
}

// GetConfigCategory retrieves all settings in a category
// category supports wildcards (e.g., "drive a*")
func (c *Client) GetConfigCategory(category string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/v1/configs/%s", url.PathEscape(category))
	resp, err := c.Get(endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.RawBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse category settings: %w", err)
	}

	return result, nil
}

// GetConfigItem retrieves detailed information about a configuration item
// Both category and item support wildcards
func (c *Client) GetConfigItem(category, item string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/v1/configs/%s/%s",
		url.PathEscape(category),
		url.PathEscape(item))
	resp, err := c.Get(endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.RawBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse config item: %w", err)
	}

	return result, nil
}

// SetConfigItem sets a specific configuration item to a new value
// Both category and item support wildcards
func (c *Client) SetConfigItem(category, item, value string) error {
	endpoint := fmt.Sprintf("/v1/configs/%s/%s?value=%s",
		url.PathEscape(category),
		url.PathEscape(item),
		url.QueryEscape(value))

	_, err := c.Put(endpoint, nil)
	return err
}

// SetMultipleConfigs changes multiple configuration settings simultaneously
// settings should be structured as: {"Category": {"Item": "Value"}}
func (c *Client) SetMultipleConfigs(settings ConfigSettings) error {
	_, err := c.PostJSON("/v1/configs", settings)
	return err
}

// LoadConfigFromFlash restores configuration from non-volatile memory
func (c *Client) LoadConfigFromFlash() error {
	_, err := c.Put("/v1/configs:load_from_flash", nil)
	return err
}

// SaveConfigToFlash writes current configuration to non-volatile memory
func (c *Client) SaveConfigToFlash() error {
	_, err := c.Put("/v1/configs:save_to_flash", nil)
	return err
}

// ResetConfigToDefault resets current settings to factory defaults
// Note: Does not affect saved values in flash
func (c *Client) ResetConfigToDefault() error {
	_, err := c.Put("/v1/configs:reset_to_default", nil)
	return err
}
