package client

import (
	"fmt"
	"strings"
)

// SetGlobalVariable executes SET GLOBAL <name> = <value>.
// The value is quoted as a string literal; StarRocks coerces it to the
// variable's underlying type (number, bool, etc.) automatically.
func (c *Client) SetGlobalVariable(name, value string) error {
	_, err := c.DB.Exec(fmt.Sprintf("SET GLOBAL %s = '%s'", name, strings.ReplaceAll(value, "'", "''")))
	return err
}

// GetGlobalVariable returns the current global value of a variable, or
// ("", false, nil) when the variable does not exist.
func (c *Client) GetGlobalVariable(name string) (string, bool, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SHOW GLOBAL VARIABLES LIKE '%s'", name))
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
	_, err := c.DB.Exec(fmt.Sprintf("SET GLOBAL %s = DEFAULT", name))
	return err
}
