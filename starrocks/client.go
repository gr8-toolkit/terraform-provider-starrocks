package starrocks

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Client struct {
	db *sql.DB
}

type ResourceGroup struct {
	Name                   types.String
	CPUWeight              types.Int64
	ExclusiveCPUCores      types.Int64
	CPUCoreLimit           types.Int64
	MaxCPUCores            types.Int64
	MemLimit               types.String
	ConcurrencyLimit       types.Int64
	BigQueryMemLimit       types.Int64
	BigQueryScanRowsLimit  types.Int64
	BigQueryCPUSecondLimit types.Int64
	Classifiers            types.List
}

type Classifier struct {
	ID        int64
	User      types.String
	Role      types.String
	QueryType types.String
	SourceIP  types.String
	DB        types.String
}

type ResourceGroupModel interface {
	GetName() types.String
	GetCPUWeight() types.Int64
	GetExclusiveCPUCores() types.Int64
	GetCPUCoreLimit() types.Int64
	GetMaxCPUCores() types.Int64
	GetMemLimit() types.String
	GetConcurrencyLimit() types.Int64
	GetBigQueryMemLimit() types.Int64
	GetBigQueryScanRowsLimit() types.Int64
	GetBigQueryCPUSecondLimit() types.Int64
	GetClassifiers() types.List
}

func NewClient(host, username, password string) (*Client, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", username, password, host)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return &Client{db: db}, nil
}

func (c *Client) CreateResourceGroup(rg ResourceGroupModel) error {
	query := fmt.Sprintf("CREATE RESOURCE GROUP `%s`", rg.GetName().ValueString())

	// Add TO clause with classifiers
	if !rg.GetClassifiers().IsNull() && len(rg.GetClassifiers().Elements()) > 0 {
		var classifierStrs []string
		for _, elem := range rg.GetClassifiers().Elements() {
			var conditions []string
			if obj, ok := elem.(types.Object); ok {
				attrs := obj.Attributes()
				if user, exists := attrs["user"]; exists && !user.IsNull() {
					if userStr, ok := user.(types.String); ok {
						conditions = append(conditions, fmt.Sprintf("user='%s'", userStr.ValueString()))
					}
				}
				if role, exists := attrs["role"]; exists && !role.IsNull() {
					if roleStr, ok := role.(types.String); ok {
						conditions = append(conditions, fmt.Sprintf("role='%s'", roleStr.ValueString()))
					}
				}
				if queryType, exists := attrs["query_type"]; exists && !queryType.IsNull() {
					if qtStr, ok := queryType.(types.String); ok {
						conditions = append(conditions, fmt.Sprintf("query_type in ('%s')", qtStr.ValueString()))
					}
				}
				if sourceIP, exists := attrs["source_ip"]; exists && !sourceIP.IsNull() {
					if sipStr, ok := sourceIP.(types.String); ok {
						conditions = append(conditions, fmt.Sprintf("source_ip='%s'", sipStr.ValueString()))
					}
				}
				if db, exists := attrs["db"]; exists && !db.IsNull() {
					if dbStr, ok := db.(types.String); ok {
						conditions = append(conditions, fmt.Sprintf("db='%s'", dbStr.ValueString()))
					}
				}
			}
			if len(conditions) > 0 {
				classifierStrs = append(classifierStrs, "("+strings.Join(conditions, ", ")+")")
			}
		}
		if len(classifierStrs) > 0 {
			query += " TO " + strings.Join(classifierStrs, ", ")
		}
	}

	// Add WITH clause with properties
	var props []string
	if !rg.GetCPUWeight().IsNull() {
		props = append(props, fmt.Sprintf("'cpu_weight' = '%d'", rg.GetCPUWeight().ValueInt64()))
	}
	if !rg.GetExclusiveCPUCores().IsNull() {
		props = append(props, fmt.Sprintf("'exclusive_cpu_cores' = '%d'", rg.GetExclusiveCPUCores().ValueInt64()))
	}
	if !rg.GetCPUCoreLimit().IsNull() {
		props = append(props, fmt.Sprintf("'cpu_core_limit' = '%d'", rg.GetCPUCoreLimit().ValueInt64()))
	}
	if !rg.GetMaxCPUCores().IsNull() {
		props = append(props, fmt.Sprintf("'max_cpu_cores' = '%d'", rg.GetMaxCPUCores().ValueInt64()))
	}
	if !rg.GetMemLimit().IsNull() {
		props = append(props, fmt.Sprintf("'mem_limit' = '%s'", rg.GetMemLimit().ValueString()))
	}
	if !rg.GetConcurrencyLimit().IsNull() {
		props = append(props, fmt.Sprintf("'concurrency_limit' = '%d'", rg.GetConcurrencyLimit().ValueInt64()))
	}
	if !rg.GetBigQueryMemLimit().IsNull() {
		props = append(props, fmt.Sprintf("'big_query_mem_limit' = '%d'", rg.GetBigQueryMemLimit().ValueInt64()))
	}
	if !rg.GetBigQueryScanRowsLimit().IsNull() {
		props = append(props, fmt.Sprintf("'big_query_scan_rows_limit' = '%d'", rg.GetBigQueryScanRowsLimit().ValueInt64()))
	}
	if !rg.GetBigQueryCPUSecondLimit().IsNull() {
		props = append(props, fmt.Sprintf("'big_query_cpu_second_limit' = '%d'", rg.GetBigQueryCPUSecondLimit().ValueInt64()))
	}

	if len(props) > 0 {
		query += " WITH (" + strings.Join(props, ", ") + ")"
	}

	_, err := c.db.Exec(query)
	return err
}

func (c *Client) GetResourceGroup(name string) (*ResourceGroup, error) {
	query := fmt.Sprintf("SHOW RESOURCE GROUP `%s`", name)
	rows, err := c.db.Query(query)
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

	rg := &ResourceGroup{Name: types.StringValue(name)}
	var classifiers []Classifier

	for rows.Next() {
		values := make([]string, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		getCol := func(name string) string {
			if idx, ok := colIndex[name]; ok {
				return values[idx]
			}
			return ""
		}

		if rg.MemLimit.IsNull() {
			if v := getCol("mem_limit"); v != "" {
				rg.MemLimit = types.StringValue(v)
			}
		}
		if rg.ConcurrencyLimit.IsNull() {
			if v, err := strconv.ParseInt(getCol("concurrency_limit"), 10, 64); err == nil && v > 0 {
				rg.ConcurrencyLimit = types.Int64Value(v)
			}
		}
		if rg.BigQueryMemLimit.IsNull() {
			if v, err := strconv.ParseInt(getCol("big_query_mem_limit"), 10, 64); err == nil && v > 0 {
				rg.BigQueryMemLimit = types.Int64Value(v)
			}
		}
		if rg.BigQueryScanRowsLimit.IsNull() {
			if v, err := strconv.ParseInt(getCol("big_query_scan_rows_limit"), 10, 64); err == nil && v > 0 {
				rg.BigQueryScanRowsLimit = types.Int64Value(v)
			}
		}
		if rg.BigQueryCPUSecondLimit.IsNull() {
			if v, err := strconv.ParseInt(getCol("big_query_cpu_second_limit"), 10, 64); err == nil && v > 0 {
				rg.BigQueryCPUSecondLimit = types.Int64Value(v)
			}
		}

		if classifiersStr := getCol("classifiers"); classifiersStr != "" {
			classifier := parseClassifier(classifiersStr)
			classifiers = append(classifiers, classifier)
		}
	}

	return rg, nil
}

func parseClassifier(s string) Classifier {
	re := regexp.MustCompile(`id=(\d+).*?user=([^,)]+)|role=([^,)]+)|query_type=([^,)]+)|source_ip=([^,)]+)|db=([^,)]+)`)
	matches := re.FindStringSubmatch(s)
	c := Classifier{}
	if len(matches) > 1 {
		c.ID, _ = strconv.ParseInt(matches[1], 10, 64)
	}
	if len(matches) > 2 && matches[2] != "" {
		c.User = types.StringValue(matches[2])
	}
	if len(matches) > 3 && matches[3] != "" {
		c.Role = types.StringValue(matches[3])
	}
	if len(matches) > 4 && matches[4] != "" {
		c.QueryType = types.StringValue(matches[4])
	}
	if len(matches) > 5 && matches[5] != "" {
		c.SourceIP = types.StringValue(matches[5])
	}
	if len(matches) > 6 && matches[6] != "" {
		c.DB = types.StringValue(matches[6])
	}
	return c
}

func (c *Client) DeleteResourceGroup(name string) error {
	query := fmt.Sprintf("DROP RESOURCE GROUP `%s`", name)
	_, err := c.db.Exec(query)
	return err
}

// ---------------------------------------------------------------------------
// Catalog
// ---------------------------------------------------------------------------

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
	_, err := c.db.Exec(query)
	return err
}

// GetCatalog returns the catalog with the given name, or nil when it does not
// exist. It uses SHOW CATALOGS LIKE '<name>' to locate the row, then
// SHOW CREATE CATALOG to recover the full properties for external catalogs.
func (c *Client) GetCatalog(name string) (*Catalog, error) {
	// Step 1: confirm the catalog exists and get its type/comment.
	rows, err := c.db.Query(fmt.Sprintf("SHOW CATALOGS LIKE '%s'", name))
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
	_, err := c.db.Exec(fmt.Sprintf("DROP CATALOG IF EXISTS `%s`", name))
	return err
}
