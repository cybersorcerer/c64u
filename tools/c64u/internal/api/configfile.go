package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// ConfigChange is one setting whose wanted value differs from the device's.
// Old is nil when the device has no such setting.
type ConfigChange struct {
	Category string
	Item     string
	Old      interface{}
	New      interface{}
}

// ParseConfigFile reads a configuration file in either of the two shapes the
// CLI knows: what config export writes, where each category holds an object of
// the same name next to an errors member, or the flat shape set-multiple reads.
func ParseConfigFile(data []byte) (ConfigSettings, error) {
	const shapes = `expected {"Category": {"Item": value}} or the output of 'c64u config export'`
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("not a configuration file: %v; %s", err, shapes)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("the file holds no categories; %s", shapes)
	}
	settings := ConfigSettings{}
	for cat, v := range raw {
		items, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("category %q is not an object; %s", cat, shapes)
		}
		// Export-Form: {"Kategorie": {"Kategorie": {...}, "errors": []}}
		if inner, ok := items[cat].(map[string]interface{}); ok {
			items = inner
		}
		settings[cat] = items
	}
	return settings, nil
}

// identityCategories and identityItems make one device distinct from another:
// copying them onto a second device would clash on the network.
var (
	identityCategories = map[string]bool{"Ethernet Settings": true, "WiFi settings": true}
	identityItems      = map[[2]string]bool{
		{"Network Settings", "Host Name"}: true,
		{"Network Settings", "Unique ID"}: true,
	}
)

// IsIdentity reports whether a setting belongs to the device's identity: its
// Ethernet and Wi-Fi addressing, host name and unique id.
func IsIdentity(category, item string) bool {
	return identityCategories[category] || identityItems[[2]string{category, item}]
}

// DiffConfig lists the settings in wanted whose value differs from current,
// sorted by category and item. Values compare as text, so 8 and 8.0 are equal.
func DiffConfig(current, wanted ConfigSettings) []ConfigChange {
	var changes []ConfigChange
	for cat, items := range wanted {
		for item, value := range items {
			old, known := current[cat][item]
			if known && fmt.Sprint(old) == fmt.Sprint(value) {
				continue
			}
			if !known {
				old = nil
			}
			changes = append(changes, ConfigChange{Category: cat, Item: item, Old: old, New: value})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Category != changes[j].Category {
			return changes[i].Category < changes[j].Category
		}
		return changes[i].Item < changes[j].Item
	})
	return changes
}

// ApplyConfig sends the changes in one request. The device tries every setting
// and names the ones it rejected; those come back as failed, the rest applied.
func (c *Client) ApplyConfig(changes []ConfigChange) (failed []string, err error) {
	if len(changes) == 0 {
		return nil, nil
	}
	body := ConfigSettings{}
	for _, ch := range changes {
		if body[ch.Category] == nil {
			body[ch.Category] = map[string]interface{}{}
		}
		body[ch.Category][ch.Item] = ch.New
	}
	resp, err := c.PostJSON("/v1/configs", body)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return nil, nil
	case http.StatusBadRequest:
		return resp.Errors, nil
	}
	return nil, fmt.Errorf("API error: %s", strings.Join(resp.Errors, ", "))
}
