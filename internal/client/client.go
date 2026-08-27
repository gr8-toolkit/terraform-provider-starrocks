package client

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// Client holds an open connection pool to a StarRocks cluster via the MySQL
// wire protocol.
type Client struct {
	DB *sql.DB
}

// NewClient opens a connection pool to the StarRocks cluster at host
// (host:port) using the given credentials. The caller must call Close when
// done with the client.
func NewClient(host, username, password string) (*Client, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", username, password, host)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return &Client{DB: db}, nil
}
