package starrocks

import (
	"fmt"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ---------------------------------------------------------------------------
// CreateDatabase
// ---------------------------------------------------------------------------

func TestCreateDatabase_simple(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("CREATE DATABASE IF NOT EXISTS `mydb`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.CreateDatabase(DatabaseDef{Name: "mydb"}); err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateDatabase_withStorageVolume(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec(`CREATE DATABASE IF NOT EXISTS ` + "`mydb`" + ` PROPERTIES`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	d := DatabaseDef{Name: "mydb", StorageVolume: "s3_vol"}
	if err := client.CreateDatabase(d); err != nil {
		t.Fatalf("CreateDatabase with storage_volume: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetDatabase
// ---------------------------------------------------------------------------

func TestGetDatabase_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectQuery("SHOW DATABASES LIKE 'mydb'").WillReturnRows(
		sqlmock.NewRows([]string{"Database"}).AddRow("mydb"),
	)

	exists, err := client.GetDatabase("mydb")
	if err != nil {
		t.Fatalf("GetDatabase: %v", err)
	}
	if !exists {
		t.Error("GetDatabase = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetDatabase_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectQuery("SHOW DATABASES LIKE 'missing'").WillReturnRows(
		sqlmock.NewRows([]string{"Database"}),
	)

	exists, err := client.GetDatabase("missing")
	if err != nil {
		t.Fatalf("GetDatabase: %v", err)
	}
	if exists {
		t.Error("GetDatabase = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateDatabase
// ---------------------------------------------------------------------------

func TestUpdateDatabase_dataQuota(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER DATABASE `mydb` SET DATA QUOTA 10G").
		WillReturnResult(sqlmock.NewResult(0, 0))

	d := DatabaseDef{Name: "mydb", DataQuota: "10G"}
	if err := client.UpdateDatabase(d); err != nil {
		t.Fatalf("UpdateDatabase data quota: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUpdateDatabase_replicaQuota(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER DATABASE `mydb` SET REPLICA QUOTA 1000").
		WillReturnResult(sqlmock.NewResult(0, 0))

	d := DatabaseDef{Name: "mydb", ReplicaQuota: 1000}
	if err := client.UpdateDatabase(d); err != nil {
		t.Fatalf("UpdateDatabase replica quota: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUpdateDatabase_allFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// All three ALTER statements must fire in order.
	mock.ExpectExec("ALTER DATABASE `mydb` SET DATA QUOTA 5G").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER DATABASE `mydb` SET REPLICA QUOTA 500").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ALTER DATABASE "mydb" SET`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	d := DatabaseDef{Name: "mydb", DataQuota: "5G", ReplicaQuota: 500, StorageVolume: "s3_vol"}
	if err := client.UpdateDatabase(d); err != nil {
		t.Fatalf("UpdateDatabase all fields: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUpdateDatabase_noOp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// Empty def — no ALTER statements should be issued.
	d := DatabaseDef{Name: "mydb"}
	if err := client.UpdateDatabase(d); err != nil {
		t.Fatalf("UpdateDatabase no-op: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DropDatabase
// ---------------------------------------------------------------------------

func TestDropDatabase_empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// SHOW TABLES returns zero rows — DB is empty, drop proceeds.
	mock.ExpectQuery("SHOW TABLES FROM `mydb`").WillReturnRows(
		sqlmock.NewRows([]string{"Tables_in_mydb"}),
	)
	mock.ExpectExec("DROP DATABASE IF EXISTS `mydb`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.DropDatabase("mydb"); err != nil {
		t.Fatalf("DropDatabase empty db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDropDatabase_nonEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// SHOW TABLES returns two tables — drop must be blocked.
	mock.ExpectQuery("SHOW TABLES FROM `mydb`").WillReturnRows(
		sqlmock.NewRows([]string{"Tables_in_mydb"}).
			AddRow("users").
			AddRow("orders"),
	)
	// DROP DATABASE must NOT be called.

	err = client.DropDatabase("mydb")
	if err == nil {
		t.Fatal("expected error when dropping non-empty database, got nil")
	}
	if !strings.Contains(err.Error(), "users") || !strings.Contains(err.Error(), "orders") {
		t.Errorf("error message should list table names, got: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDropDatabase_oneTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectQuery("SHOW TABLES FROM `mydb`").WillReturnRows(
		sqlmock.NewRows([]string{"Tables_in_mydb"}).AddRow("events"),
	)

	err = client.DropDatabase("mydb")
	if err == nil {
		t.Fatal("expected error for database with 1 table")
	}
	if !strings.Contains(err.Error(), "1 table") {
		t.Errorf("error should mention '1 table', got: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// isDatabaseNotFoundError
// ---------------------------------------------------------------------------

func TestIsDatabaseNotFoundError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"Error 1049: Unknown database 'mydb'", true},
		{"database mydb does not exist", true},
		{"mydb is not found", true},
		{"some other error", false},
		{"table t does not exist", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := isDatabaseNotFoundError(fmt.Errorf("%s", tt.msg))
			if got != tt.want {
				t.Errorf("isDatabaseNotFoundError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}
