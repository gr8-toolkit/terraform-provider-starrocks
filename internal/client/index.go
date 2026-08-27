package client

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// IndexDef holds the definition of a StarRocks secondary index.
type IndexDef struct {
	Name       string
	Column     string
	Type       string            // BITMAP | NGRAMBF | GIN | VECTOR
	Comment    string
	Properties map[string]string // optional; used for NGRAMBF/GIN/VECTOR params
}

// sleepFn is a variable so tests can replace it to avoid real sleeps.
var sleepFn = func(d time.Duration) { time.Sleep(d) }

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

	if _, err := c.DB.Exec(q); err != nil {
		return err
	}

	return c.waitForIndexJob(db, table, idx.Name, timeoutSec)
}

// GetIndex returns the index with the given name on the table, or nil if it
// does not exist. It queries SHOW INDEXES FROM db.table and scans the result.
func (c *Client) GetIndex(db, table, name string) (*IndexDef, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SHOW INDEXES FROM `%s`.`%s`", db, table))
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
	_, err := c.DB.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`.`%s`", name, db, table))
	if err != nil {
		// Treat not-found as success.
		if IsIndexNotFoundError(err) {
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
		rows, err := c.DB.Query(fmt.Sprintf(
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

// IsIndexNotFoundError reports whether err indicates an index does not exist.
func IsIndexNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "is not found") ||
		strings.Contains(msg, "index") && strings.Contains(msg, "not found")
}
