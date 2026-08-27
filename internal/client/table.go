package client

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ColumnDef holds the definition of a single table column as Terraform sees it.
type ColumnDef struct {
	Name     string
	Type     string
	Nullable bool   // true means NULL is allowed
	Default  string // empty string means no default
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

	_, err := c.DB.Exec(q)
	return err
}

// GetTable returns the current column definitions of a table by parsing
// SHOW CREATE TABLE. Returns nil when the table does not exist.
func (c *Client) GetTable(db, name string) (*TableDef, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", db, name))
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
	_, err := c.DB.Exec(fmt.Sprintf(
		"ALTER TABLE `%s`.`%s` ADD COLUMN %s",
		db, table, buildColumnDDL(col),
	))
	return err
}

// AlterTableDropColumn issues ALTER TABLE ... DROP COLUMN for a single column.
func (c *Client) AlterTableDropColumn(db, table, column string) error {
	_, err := c.DB.Exec(fmt.Sprintf(
		"ALTER TABLE `%s`.`%s` DROP COLUMN `%s`",
		db, table, column,
	))
	return err
}

// AlterTableModifyColumn issues ALTER TABLE ... MODIFY COLUMN to change
// a column's type, nullability, default, or comment.
func (c *Client) AlterTableModifyColumn(db, table string, col ColumnDef) error {
	_, err := c.DB.Exec(fmt.Sprintf(
		"ALTER TABLE `%s`.`%s` MODIFY COLUMN %s",
		db, table, buildColumnDDL(col),
	))
	return err
}

// AlterTableComment issues ALTER TABLE ... COMMENT to update the table-level
// comment without recreating the table.
func (c *Client) AlterTableComment(db, table, comment string) error {
	_, err := c.DB.Exec(fmt.Sprintf(
		"ALTER TABLE `%s`.`%s` COMMENT = %q",
		db, table, comment,
	))
	return err
}

// DropTable issues DROP TABLE IF EXISTS.
func (c *Client) DropTable(db, name string) error {
	_, err := c.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`.`%s`", db, name))
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
