package client

import (
	"fmt"
	"sort"
	"strings"
)

// Plugin holds the data returned by SHOW PLUGINS for a single plugin row.
type Plugin struct {
	Name        string
	Type        string
	Description string
	Version     string
	JavaVersion string
	ClassName   string
	SoName      string
	Sources     string
	Status      string
	Properties  string
}

// InstallPlugin executes INSTALL PLUGIN FROM "<source>" with optional
// PROPERTIES. The properties map may be nil or empty when no properties are
// needed.
func (c *Client) InstallPlugin(source string, properties map[string]string) error {
	q := fmt.Sprintf("INSTALL PLUGIN FROM %q", source)
	if len(properties) > 0 {
		var pairs []string
		for k, v := range properties {
			pairs = append(pairs, fmt.Sprintf("%q = %q", k, v))
		}
		// Sort for deterministic SQL (important for unit-test expectations).
		sort.Strings(pairs)
		q += " PROPERTIES (" + strings.Join(pairs, ", ") + ")"
	}
	_, err := c.DB.Exec(q)
	return err
}

// GetPlugin returns the plugin with the given name from SHOW PLUGINS, or nil
// when no such plugin exists. Name comparison is case-insensitive.
func (c *Client) GetPlugin(name string) (*Plugin, error) {
	rows, err := c.DB.Query("SHOW PLUGINS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	colIndex := make(map[string]int, len(cols))
	for i, col := range cols {
		colIndex[strings.ToLower(col)] = i
	}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		getString := func(col string) string {
			idx, ok := colIndex[col]
			if !ok {
				return ""
			}
			if values[idx] == nil {
				return ""
			}
			switch v := values[idx].(type) {
			case []byte:
				return string(v)
			case string:
				return v
			}
			return fmt.Sprintf("%v", values[idx])
		}

		if !strings.EqualFold(getString("name"), name) {
			continue
		}

		return &Plugin{
			Name:        getString("name"),
			Type:        getString("type"),
			Description: getString("description"),
			Version:     getString("version"),
			JavaVersion: getString("javaversion"),
			ClassName:   getString("classname"),
			SoName:      getString("soname"),
			Sources:     getString("sources"),
			Status:      getString("status"),
			Properties:  getString("properties"),
		}, nil
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

// UninstallPlugin executes UNINSTALL PLUGIN <plugin_name>.
// Only non-builtin plugins can be uninstalled.
func (c *Client) UninstallPlugin(name string) error {
	_, err := c.DB.Exec(fmt.Sprintf("UNINSTALL PLUGIN %s", name))
	return err
}
