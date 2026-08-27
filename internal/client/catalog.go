package client

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Catalog holds the data returned by SHOW CATALOGS for a single row.
type Catalog struct {
	Name    string
	Type    string
	Comment types.String
}

// CreateCatalog executes CREATE EXTERNAL CATALOG. The properties map must
// contain at least "type". An optional comment is included when non-empty.
// The internal catalog (default_catalog) cannot be created via SQL — callers
// must never pass it here.
func (c *Client) CreateCatalog(name, comment string, properties map[string]string) error {
	query := fmt.Sprintf("CREATE EXTERNAL CATALOG `%s`", name)
	if comment != "" {
		query += fmt.Sprintf(" COMMENT %q", comment)
	}
	if len(properties) > 0 {
		var pairs []string
		for k, v := range properties {
			pairs = append(pairs, fmt.Sprintf("%q = %q", k, v))
		}
		// Sort for deterministic SQL (important for unit-test expectations).
		sort.Strings(pairs)
		query += " PROPERTIES (" + strings.Join(pairs, ", ") + ")"
	}
	_, err := c.DB.Exec(query)
	return err
}

// GetCatalog returns the catalog with the given name, or nil when it does not
// exist. It uses SHOW CATALOGS LIKE '<name>' to locate the row.
func (c *Client) GetCatalog(name string) (*Catalog, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SHOW CATALOGS LIKE '%s'", name))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cat *Catalog
	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		colIndex := make(map[string]int, len(cols))
		for i, col := range cols {
			colIndex[strings.ToLower(col)] = i
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
		catalogName := getString("catalog")
		if !strings.EqualFold(catalogName, name) {
			continue
		}
		comment := getString("comment")
		cat = &Catalog{
			Name:    catalogName,
			Type:    getString("type"),
			Comment: types.StringValue(comment),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cat, nil
}

// DeleteCatalog executes DROP CATALOG IF EXISTS. The internal catalog
// (default_catalog) cannot be deleted — callers must guard against passing it.
func (c *Client) DeleteCatalog(name string) error {
	_, err := c.DB.Exec(fmt.Sprintf("DROP CATALOG IF EXISTS `%s`", name))
	return err
}
