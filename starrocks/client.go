package starrocks

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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

// ---------------------------------------------------------------------------
// Table
// ---------------------------------------------------------------------------

// ColumnDef holds the definition of a single table column as Terraform sees it.
type ColumnDef struct {
	Name     string
	Type     string
	Nullable bool   // true means NULL is allowed
	Default  string // empty string means no default; use HasDefault to distinguish
	Comment  string
}

// TableDef is the parsed representation of a StarRocks table used in state.
type TableDef struct {
	Database   string
	Name       string
	Engine     string
	KeyType    string
	KeyColumns []string
	Columns    []ColumnDef
	DistBy     string
	Comment    string
	Properties map[string]string
}

// CreateTable executes CREATE TABLE. The caller provides the full column list,
// key description, distribution clause, optional comment, and optional properties.
func (c *Client) CreateTable(db string, t *TableDef) error {
	var colDefs []string
	for _, col := range t.Columns {
		colDefs = append(colDefs, buildColumnDDL(col))
	}

	q := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` (\n  %s\n)",
		db, t.Name, strings.Join(colDefs, ",\n  "))

	if t.Engine != "" {
		q += "\nENGINE = " + t.Engine
	}
	if t.KeyType != "" && len(t.KeyColumns) > 0 {
		backticked := make([]string, len(t.KeyColumns))
		for i, k := range t.KeyColumns {
			backticked[i] = "`" + k + "`"
		}
		q += fmt.Sprintf("\n%s (%s)", t.KeyType, strings.Join(backticked, ", "))
	}
	if t.Comment != "" {
		q += fmt.Sprintf("\nCOMMENT %q", t.Comment)
	}
	if t.DistBy != "" {
		q += "\n" + t.DistBy
	}
	if len(t.Properties) > 0 {
		var pairs []string
		for k, v := range t.Properties {
			pairs = append(pairs, fmt.Sprintf("%q = %q", k, v))
		}
		sort.Strings(pairs)
		q += "\nPROPERTIES (" + strings.Join(pairs, ", ") + ")"
	}

	_, err := c.db.Exec(q)
	return err
}

// GetTable returns the current column definitions of a table by parsing
// SHOW CREATE TABLE. Returns nil when the table does not exist.
func (c *Client) GetTable(db, name string) (*TableDef, error) {
	rows, err := c.db.Query(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", db, name))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableName, createSQL string
	if rows.Next() {
		if err := rows.Scan(&tableName, &createSQL); err != nil {
			return nil, err
		}
	}
	if createSQL == "" {
		return nil, nil
	}

	return parseCreateTable(db, createSQL)
}

// AlterTableAddColumn issues ALTER TABLE ... ADD COLUMN for a single column.
func (c *Client) AlterTableAddColumn(db, table string, col ColumnDef) error {
	_, err := c.db.Exec(fmt.Sprintf(
		"ALTER TABLE `%s`.`%s` ADD COLUMN %s",
		db, table, buildColumnDDL(col),
	))
	return err
}

// AlterTableDropColumn issues ALTER TABLE ... DROP COLUMN for a single column.
func (c *Client) AlterTableDropColumn(db, table, column string) error {
	_, err := c.db.Exec(fmt.Sprintf(
		"ALTER TABLE `%s`.`%s` DROP COLUMN `%s`",
		db, table, column,
	))
	return err
}

// AlterTableModifyColumn issues ALTER TABLE ... MODIFY COLUMN to change
// a column's type, nullability, default, or comment.
func (c *Client) AlterTableModifyColumn(db, table string, col ColumnDef) error {
	_, err := c.db.Exec(fmt.Sprintf(
		"ALTER TABLE `%s`.`%s` MODIFY COLUMN %s",
		db, table, buildColumnDDL(col),
	))
	return err
}

// DropTable issues DROP TABLE IF EXISTS.
func (c *Client) DropTable(db, name string) error {
	_, err := c.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`.`%s`", db, name))
	return err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildColumnDDL renders a ColumnDef to a DDL fragment, e.g.:
//
//	`id` INT NOT NULL DEFAULT "0" COMMENT "primary key"
func buildColumnDDL(col ColumnDef) string {
	s := fmt.Sprintf("`%s` %s", col.Name, col.Type)
	if col.Nullable {
		s += " NULL"
	} else {
		s += " NOT NULL"
	}
	if col.Default != "" {
		s += fmt.Sprintf(" DEFAULT %q", col.Default)
	}
	if col.Comment != "" {
		s += fmt.Sprintf(" COMMENT %q", col.Comment)
	}
	return s
}

// parseCreateTable parses the output of SHOW CREATE TABLE into a TableDef.
// It only extracts the column list; other fields (ENGINE, key type, etc.)
// are populated from what StarRocks returns.
func parseCreateTable(db, ddl string) (*TableDef, error) {
	t := &TableDef{Database: db, Properties: make(map[string]string)}

	// Table name
	if m := regexp.MustCompile("CREATE TABLE `([^`]+)`").FindStringSubmatch(ddl); len(m) > 1 {
		t.Name = m[1]
	}

	// ENGINE
	if m := regexp.MustCompile(`(?i)ENGINE\s*=\s*(\w+)`).FindStringSubmatch(ddl); len(m) > 1 {
		t.Engine = m[1]
	}

	// COMMENT at table level (after ENGINE / key desc, before DISTRIBUTED)
	if m := regexp.MustCompile(`(?i)COMMENT\s+"([^"]*)"[\s\n]*(?:DISTRIBUTED|PROPERTIES|$)`).FindStringSubmatch(ddl); len(m) > 1 {
		t.Comment = m[1]
	}

	// DISTRIBUTED BY clause — capture everything up to PROPERTIES or end
	if m := regexp.MustCompile(`(?i)(DISTRIBUTED BY[^\n]+(?:BUCKETS\s+\d+)?)`).FindStringSubmatch(ddl); len(m) > 1 {
		t.DistBy = strings.TrimSpace(m[1])
	}

	// Key type and key columns
	if m := regexp.MustCompile(`(?i)(DUPLICATE KEY|AGGREGATE KEY|UNIQUE KEY|PRIMARY KEY)\s*\(([^)]+)\)`).FindStringSubmatch(ddl); len(m) > 2 {
		t.KeyType = strings.ToUpper(strings.TrimSpace(m[1]))
		for _, k := range strings.Split(m[2], ",") {
			k = strings.TrimSpace(k)
			k = strings.Trim(k, "`")
			t.KeyColumns = append(t.KeyColumns, k)
		}
	}

	// PROPERTIES block
	if m := regexp.MustCompile(`(?i)PROPERTIES\s*\(([^)]+)\)`).FindStringSubmatch(ddl); len(m) > 1 {
		re := regexp.MustCompile(`"([^"]+)"\s*=\s*"([^"]*)"`)
		for _, pair := range re.FindAllStringSubmatch(m[1], -1) {
			t.Properties[pair[1]] = pair[2]
		}
	}

	// Columns — extract the block between the first ( and the matching )
	// then parse line by line.
	colBlock := extractColumnBlock(ddl)
	for _, line := range strings.Split(colBlock, "\n") {
		col, ok := parseColumnLine(line)
		if !ok {
			continue
		}
		t.Columns = append(t.Columns, col)
	}

	return t, nil
}

// extractColumnBlock returns the text inside the outermost parentheses of the
// CREATE TABLE statement (i.e., the column/index definitions block).
func extractColumnBlock(ddl string) string {
	start := strings.Index(ddl, "(")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(ddl); i++ {
		switch ddl[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return ddl[start+1 : i]
			}
		}
	}
	return ""
}

// parseColumnLine parses a single line from the column block of SHOW CREATE TABLE.
// Returns false for lines that define indexes, keys, or are empty.
//
// StarRocks output example:
//
//	`id` int(11) NOT NULL COMMENT "pk"
//	`name` varchar(128) NULL DEFAULT "anon" COMMENT ""
func parseColumnLine(line string) (ColumnDef, bool) {
	line = strings.TrimSpace(line)
	// Skip empty lines, index definitions, and key definitions
	if line == "" ||
		strings.HasPrefix(strings.ToUpper(line), "INDEX") ||
		strings.HasPrefix(strings.ToUpper(line), "UNIQUE") ||
		strings.HasPrefix(strings.ToUpper(line), "PRIMARY") ||
		strings.HasPrefix(strings.ToUpper(line), "KEY") {
		return ColumnDef{}, false
	}
	// Must start with a backtick-quoted name
	if !strings.HasPrefix(line, "`") {
		return ColumnDef{}, false
	}

	// Strip trailing comma
	line = strings.TrimRight(line, ",")

	col := ColumnDef{Nullable: true} // StarRocks default

	// Name
	nameEnd := strings.Index(line[1:], "`")
	if nameEnd < 0 {
		return ColumnDef{}, false
	}
	col.Name = line[1 : nameEnd+1]
	rest := strings.TrimSpace(line[nameEnd+2:])

	// Comment (greedy from the end)
	if m := regexp.MustCompile(`COMMENT\s+"([^"]*)"\s*$`).FindStringSubmatchIndex(rest); m != nil {
		col.Comment = rest[m[2]:m[3]]
		rest = strings.TrimSpace(rest[:m[0]])
	}

	// Default value
	if m := regexp.MustCompile(`DEFAULT\s+"([^"]*)"\s*$`).FindStringSubmatchIndex(rest); m != nil {
		col.Default = rest[m[2]:m[3]]
		rest = strings.TrimSpace(rest[:m[0]])
	}

	// Nullability
	upperRest := strings.ToUpper(rest)
	if strings.HasSuffix(upperRest, "NOT NULL") {
		col.Nullable = false
		rest = strings.TrimSpace(rest[:len(rest)-8])
	} else if strings.HasSuffix(upperRest, "NULL") {
		col.Nullable = true
		rest = strings.TrimSpace(rest[:len(rest)-4])
	}

	// Whatever remains is the type
	col.Type = normaliseColumnType(strings.TrimSpace(rest))
	if col.Type == "" {
		return ColumnDef{}, false
	}

	return col, true
}

// normaliseColumnType converts a type string returned by StarRocks
// (e.g. "bigint(20)", "varchar(1024)", "tinyint(4)") to the canonical form
// the user writes in config (e.g. "BIGINT", "VARCHAR(1024)", "TINYINT").
//
// Rules:
//  1. Uppercase the whole string.
//  2. For integer types that StarRocks annotates with a display-width suffix
//     (TINYINT, SMALLINT, INT, BIGINT, LARGEINT) the suffix is meaningless and
//     is stripped so the value matches what the user wrote.
func normaliseColumnType(t string) string {
	t = strings.ToUpper(strings.TrimSpace(t))

	// Strip display-width from fixed-width integer types.
	displayWidthIntegers := regexp.MustCompile(
		`^(TINYINT|SMALLINT|INT|BIGINT|LARGEINT)\(\d+\)$`,
	)
	if m := displayWidthIntegers.FindStringSubmatch(t); len(m) > 1 {
		return m[1]
	}

	return t
}

// ---------------------------------------------------------------------------
// Index
// ---------------------------------------------------------------------------

// IndexDef holds the definition of a StarRocks secondary index.
type IndexDef struct {
	Name       string
	Column     string
	Type       string // BITMAP | NGRAMBF | GIN | VECTOR
	Comment    string
	Properties map[string]string // optional; used for NGRAMBF/GIN/VECTOR params
}

// CreateIndex issues ALTER TABLE ... ADD INDEX and then waits for the
// asynchronous schema-change job to reach FINISHED or CANCELLED state.
// It returns when the job completes or after the given timeout.
func (c *Client) CreateIndex(db, table string, idx IndexDef, timeoutSec int) error {
	q := fmt.Sprintf("ALTER TABLE `%s`.`%s` ADD INDEX `%s` (`%s`) USING %s",
		db, table, idx.Name, idx.Column, idx.Type)

	if len(idx.Properties) > 0 {
		var pairs []string
		for k, v := range idx.Properties {
			pairs = append(pairs, fmt.Sprintf("%q = %q", k, v))
		}
		sort.Strings(pairs)
		q += " (" + strings.Join(pairs, ", ") + ")"
	}
	if idx.Comment != "" {
		q += fmt.Sprintf(" COMMENT %q", idx.Comment)
	}

	if _, err := c.db.Exec(q); err != nil {
		return err
	}

	return c.waitForIndexJob(db, table, idx.Name, timeoutSec)
}

// GetIndex returns the index with the given name on the table, or nil if it
// does not exist. It queries SHOW INDEXES FROM db.table and scans the result.
func (c *Client) GetIndex(db, table, name string) (*IndexDef, error) {
	rows, err := c.db.Query(fmt.Sprintf("SHOW INDEXES FROM `%s`.`%s`", db, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			var s sql.NullString
			values[i] = &s
			ptrs[i] = &s
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		get := func(col string) string {
			for i, c := range cols {
				if strings.EqualFold(c, col) {
					if ns, ok := values[i].(*sql.NullString); ok && ns.Valid {
						return ns.String
					}
				}
			}
			return ""
		}

		if !strings.EqualFold(get("Key_name"), name) {
			continue
		}

		indexType := strings.ToUpper(get("Index_type"))
		// StarRocks encodes resolved properties inside the Index_type column for
		// NGRAMBF, GIN, and VECTOR indexes, e.g.:
		//   NGRAMBF("GRAM_NUM" = "4", "BLOOM_FILTER_FPP" = "0.05", ...)
		// Strip the property suffix so the returned type always matches what the
		// user wrote in config (e.g. "NGRAMBF").
		if i := strings.IndexByte(indexType, '('); i >= 0 {
			indexType = strings.TrimSpace(indexType[:i])
		}

		return &IndexDef{
			Name:    get("Key_name"),
			Column:  get("Column_name"),
			Type:    indexType,
			Comment: get("Comment"),
		}, nil
	}
	return nil, rows.Err()
}

// DropIndex issues DROP INDEX ... ON db.table and waits for the async job.
func (c *Client) DropIndex(db, table, name string, timeoutSec int) error {
	_, err := c.db.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`.`%s`", name, db, table))
	if err != nil {
		// Treat not-found as success.
		if isIndexNotFoundError(err) {
			return nil
		}
		return err
	}
	return c.waitForIndexJob(db, table, name, timeoutSec)
}

// waitForIndexJob polls SHOW ALTER TABLE COLUMN until the schema-change job
// for the given index reaches a terminal state (FINISHED or CANCELLED).
// It returns an error if the job is CANCELLED or if the timeout is exceeded.
func (c *Client) waitForIndexJob(db, table, indexName string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 300 // default 5 min
	}
	deadline := timeoutSec * 10 // poll every 100 ms → N iterations

	for i := 0; i < deadline; i++ {
		rows, err := c.db.Query(fmt.Sprintf(
			"SHOW ALTER TABLE COLUMN FROM `%s` WHERE TableName = '%s'",
			db, table,
		))
		if err != nil {
			return err
		}

		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return err
		}

		var lastState string
		for rows.Next() {
			values := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for j := range values {
				var s sql.NullString
				values[j] = &s
				ptrs[j] = &s
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return err
			}
			get := func(col string) string {
				for k, c := range cols {
					if strings.EqualFold(c, col) {
						if ns, ok := values[k].(*sql.NullString); ok && ns.Valid {
							return ns.String
						}
					}
				}
				return ""
			}
			lastState = get("State")
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		switch strings.ToUpper(lastState) {
		case "FINISHED":
			return nil
		case "CANCELLED":
			return fmt.Errorf("index schema-change job was cancelled")
		}

		// Still running — wait 100 ms before the next poll.
		sleepFn(100 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for index job on %s.%s after %ds", db, table, timeoutSec)
}

// sleepFn is a variable so tests can replace it to avoid real sleeps.
var sleepFn = func(d time.Duration) { time.Sleep(d) }

// isIndexNotFoundError reports whether err indicates an index does not exist.
func isIndexNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "is not found") ||
		strings.Contains(msg, "index") && strings.Contains(msg, "not found")
}

// ---------------------------------------------------------------------------
// Database
// ---------------------------------------------------------------------------

// DatabaseDef holds the mutable properties of a StarRocks database.
// SHOW CREATE DATABASE returns only the name, so data_quota, replica_quota,
// and storage_volume cannot be read back from the server — they are kept in
// Terraform state as write-only values.
type DatabaseDef struct {
	Name          string
	DataQuota     string // e.g. "10G", "" means unset
	ReplicaQuota  int64  // 0 means unset
	StorageVolume string // "" means unset
}

// CreateDatabase executes CREATE DATABASE IF NOT EXISTS and optionally sets
// storage_volume via PROPERTIES.
func (c *Client) CreateDatabase(d DatabaseDef) error {
	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", d.Name)
	if d.StorageVolume != "" {
		q += fmt.Sprintf(` PROPERTIES ("storage_volume" = "%s")`, d.StorageVolume)
	}
	_, err := c.db.Exec(q)
	return err
}

// GetDatabase returns true when a database with the given name exists.
// It uses SHOW DATABASES LIKE because SHOW CREATE DATABASE doesn't expose
// properties and errors loudly on unknown databases.
func (c *Client) GetDatabase(name string) (bool, error) {
	rows, err := c.db.Query(fmt.Sprintf("SHOW DATABASES LIKE '%s'", name))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return false, err
		}
		if strings.EqualFold(dbName, name) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// UpdateDatabase applies in-place changes that StarRocks supports without
// recreating the database (data quota, replica quota, storage volume).
// It only issues ALTER statements for fields that are non-empty / non-zero.
func (c *Client) UpdateDatabase(d DatabaseDef) error {
	if d.DataQuota != "" {
		if _, err := c.db.Exec(fmt.Sprintf(
			"ALTER DATABASE `%s` SET DATA QUOTA %s", d.Name, d.DataQuota,
		)); err != nil {
			return fmt.Errorf("setting data quota: %w", err)
		}
	}
	if d.ReplicaQuota > 0 {
		if _, err := c.db.Exec(fmt.Sprintf(
			"ALTER DATABASE `%s` SET REPLICA QUOTA %d", d.Name, d.ReplicaQuota,
		)); err != nil {
			return fmt.Errorf("setting replica quota: %w", err)
		}
	}
	if d.StorageVolume != "" {
		if _, err := c.db.Exec(fmt.Sprintf(
			`ALTER DATABASE "%s" SET ("storage_volume" = "%s")`, d.Name, d.StorageVolume,
		)); err != nil {
			return fmt.Errorf("setting storage volume: %w", err)
		}
	}
	return nil
}

// DropDatabase executes DROP DATABASE IF EXISTS.
// It returns an error if the database still contains tables, to prevent
// accidental data loss. StarRocks silently drops non-empty databases without
// any warning, so this check must be done in the provider.
func (c *Client) DropDatabase(name string) error {
	tables, err := c.ListDatabaseTables(name)
	if err != nil {
		return fmt.Errorf("checking tables before drop: %w", err)
	}
	if len(tables) > 0 {
		return fmt.Errorf(
			"database %q still contains %d table(s) (%s): remove all tables before deleting the database",
			name, len(tables), strings.Join(tables, ", "),
		)
	}
	_, err = c.db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	return err
}

// ListDatabaseTables returns the names of all tables in the given database.
// Returns an empty slice when the database is empty or does not exist.
func (c *Client) ListDatabaseTables(name string) ([]string, error) {
	rows, err := c.db.Query(fmt.Sprintf("SHOW TABLES FROM `%s`", name))
	if err != nil {
		// If the database doesn't exist SHOW TABLES returns an error; treat
		// that as an empty list so DropDatabase can proceed.
		if isDatabaseNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// ---------------------------------------------------------------------------
// Global variable
// ---------------------------------------------------------------------------

// SetGlobalVariable executes SET GLOBAL <name> = <value>.
// The value is quoted as a string literal; StarRocks coerces it to the
// variable's underlying type (number, bool, etc.) automatically.
func (c *Client) SetGlobalVariable(name, value string) error {
	_, err := c.db.Exec(fmt.Sprintf("SET GLOBAL %s = '%s'", name, strings.ReplaceAll(value, "'", "''")))
	return err
}

// GetGlobalVariable returns the current global value of a variable, or
// ("", false, nil) when the variable does not exist.
func (c *Client) GetGlobalVariable(name string) (string, bool, error) {
	rows, err := c.db.Query(fmt.Sprintf("SHOW GLOBAL VARIABLES LIKE '%s'", name))
	if err != nil {
		return "", false, err
	}
	defer rows.Close()

	for rows.Next() {
		var varName, varValue string
		if err := rows.Scan(&varName, &varValue); err != nil {
			return "", false, err
		}
		// LIKE matches by pattern; ensure we got the exact variable.
		if strings.EqualFold(varName, name) {
			return varValue, true, nil
		}
	}
	return "", false, rows.Err()
}

// ResetGlobalVariable resets a global variable to its default value by
// executing SET GLOBAL <name> = DEFAULT.
func (c *Client) ResetGlobalVariable(name string) error {
	_, err := c.db.Exec(fmt.Sprintf("SET GLOBAL %s = DEFAULT", name))
	return err
}
