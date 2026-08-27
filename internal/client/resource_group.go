package client

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ResourceGroup holds the data returned by SHOW RESOURCE GROUP for a single
// resource group.
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

// Classifier holds one classifier row returned by SHOW RESOURCE GROUP.
type Classifier struct {
	ID        int64
	User      types.String
	Role      types.String
	QueryType types.String
	SourceIP  types.String
	DB        types.String
}

// ResourceGroupModel is an interface that decouples the client's
// CreateResourceGroup from the Terraform resource model type. The Terraform
// resource model implements this interface so the client can be tested without
// importing terraform-plugin-framework resource structs.
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

// CreateResourceGroup executes CREATE RESOURCE GROUP with optional classifiers
// (TO clause) and properties (WITH clause).
func (c *Client) CreateResourceGroup(rg ResourceGroupModel) error {
	query := fmt.Sprintf("CREATE RESOURCE GROUP `%s`", rg.GetName().ValueString())

	// Build TO clause from classifiers.
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

	// Build WITH clause from properties.
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

	_, err := c.DB.Exec(query)
	return err
}

// GetResourceGroup executes SHOW RESOURCE GROUP and returns the parsed result,
// or nil when the resource group does not exist.
func (c *Client) GetResourceGroup(name string) (*ResourceGroup, error) {
	query := fmt.Sprintf("SHOW RESOURCE GROUP `%s`", name)
	rows, err := c.DB.Query(query)
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

		getCol := func(colName string) string {
			if idx, ok := colIndex[colName]; ok {
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

	// classifiers slice is populated but not stored on the struct (intentional
	// — classifiers are kept from planned state, not re-read from the server,
	// to avoid parser fragility).
	_ = classifiers

	return rg, nil
}

// DeleteResourceGroup executes DROP RESOURCE GROUP.
func (c *Client) DeleteResourceGroup(name string) error {
	query := fmt.Sprintf("DROP RESOURCE GROUP `%s`", name)
	_, err := c.DB.Exec(query)
	return err
}

// parseClassifier parses the classifiers column returned by SHOW RESOURCE GROUP.
func parseClassifier(s string) Classifier {
	re := regexp.MustCompile(`id=(\d+).*?user=([^,)]+)|role=([^,)]+)|query_type=([^,)]+)|source_ip=([^,)]+)|db=([^,)]+)`)
	matches := re.FindStringSubmatch(s)
	cl := Classifier{}
	if len(matches) > 1 {
		cl.ID, _ = strconv.ParseInt(matches[1], 10, 64)
	}
	if len(matches) > 2 && matches[2] != "" {
		cl.User = types.StringValue(matches[2])
	}
	if len(matches) > 3 && matches[3] != "" {
		cl.Role = types.StringValue(matches[3])
	}
	if len(matches) > 4 && matches[4] != "" {
		cl.QueryType = types.StringValue(matches[4])
	}
	if len(matches) > 5 && matches[5] != "" {
		cl.SourceIP = types.StringValue(matches[5])
	}
	if len(matches) > 6 && matches[6] != "" {
		cl.DB = types.StringValue(matches[6])
	}
	return cl
}
