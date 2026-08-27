package client

import (
	"fmt"
	"strings"
)

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
	_, err := c.DB.Exec(q)
	return err
}

// GetDatabase returns true when a database with the given name exists.
// It uses SHOW DATABASES LIKE because SHOW CREATE DATABASE doesn't expose
// properties and errors loudly on unknown databases.
func (c *Client) GetDatabase(name string) (bool, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SHOW DATABASES LIKE '%s'", name))
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
		if _, err := c.DB.Exec(fmt.Sprintf(
			"ALTER DATABASE `%s` SET DATA QUOTA %s", d.Name, d.DataQuota,
		)); err != nil {
			return fmt.Errorf("setting data quota: %w", err)
		}
	}
	if d.ReplicaQuota > 0 {
		if _, err := c.DB.Exec(fmt.Sprintf(
			"ALTER DATABASE `%s` SET REPLICA QUOTA %d", d.Name, d.ReplicaQuota,
		)); err != nil {
			return fmt.Errorf("setting replica quota: %w", err)
		}
	}
	if d.StorageVolume != "" {
		if _, err := c.DB.Exec(fmt.Sprintf(
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
	_, err = c.DB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	return err
}

// ListDatabaseTables returns the names of all tables in the given database.
// Returns an empty slice when the database is empty or does not exist.
func (c *Client) ListDatabaseTables(name string) ([]string, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SHOW TABLES FROM `%s`", name))
	if err != nil {
		// If the database doesn't exist SHOW TABLES returns an error; treat
		// that as an empty list so DropDatabase can proceed.
		if IsDatabaseNotFoundError(err) {
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

// IsDatabaseNotFoundError reports whether err indicates that a database does
// not exist.
func IsDatabaseNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown database") ||
		strings.Contains(msg, "database") && strings.Contains(msg, "not exist") ||
		strings.Contains(msg, "is not found")
}
