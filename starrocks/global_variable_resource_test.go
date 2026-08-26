package starrocks

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ---------------------------------------------------------------------------
// SetGlobalVariable
// ---------------------------------------------------------------------------

func TestSetGlobalVariable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("SET GLOBAL query_timeout = '600'").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.SetGlobalVariable("query_timeout", "600"); err != nil {
		t.Fatalf("SetGlobalVariable: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSetGlobalVariable_escapesQuotes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// Single quotes in the value must be escaped as ''.
	mock.ExpectExec(`SET GLOBAL time_zone = 'Asia/Shanghai'`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.SetGlobalVariable("time_zone", "Asia/Shanghai"); err != nil {
		t.Fatalf("SetGlobalVariable with special value: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetGlobalVariable
// ---------------------------------------------------------------------------

func TestGetGlobalVariable_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectQuery("SHOW GLOBAL VARIABLES LIKE 'query_timeout'").WillReturnRows(
		sqlmock.NewRows([]string{"Variable_name", "Value"}).
			AddRow("query_timeout", "600"),
	)

	val, exists, err := client.GetGlobalVariable("query_timeout")
	if err != nil {
		t.Fatalf("GetGlobalVariable: %v", err)
	}
	if !exists {
		t.Fatal("exists = false, want true")
	}
	if val != "600" {
		t.Errorf("value = %q, want 600", val)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetGlobalVariable_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// SHOW GLOBAL VARIABLES LIKE returns zero rows for unknown variables.
	mock.ExpectQuery("SHOW GLOBAL VARIABLES LIKE 'nonexistent_var'").WillReturnRows(
		sqlmock.NewRows([]string{"Variable_name", "Value"}),
	)

	_, exists, err := client.GetGlobalVariable("nonexistent_var")
	if err != nil {
		t.Fatalf("GetGlobalVariable: %v", err)
	}
	if exists {
		t.Error("exists = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetGlobalVariable_likePartialMatch verifies that GetGlobalVariable only
// returns true when the variable name is an exact (case-insensitive) match,
// not a LIKE partial match (e.g. querying "query_timeout" should not match
// "query_timeout_ms" if that were ever returned by the LIKE pattern).
func TestGetGlobalVariable_likePartialMatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// Simulate a row whose name doesn't exactly match the requested name.
	mock.ExpectQuery("SHOW GLOBAL VARIABLES LIKE 'query_timeout'").WillReturnRows(
		sqlmock.NewRows([]string{"Variable_name", "Value"}).
			AddRow("query_timeout_ms", "300000"),
	)

	_, exists, err := client.GetGlobalVariable("query_timeout")
	if err != nil {
		t.Fatalf("GetGlobalVariable: %v", err)
	}
	if exists {
		t.Error("exists = true for partial match, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ResetGlobalVariable
// ---------------------------------------------------------------------------

func TestResetGlobalVariable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("SET GLOBAL query_timeout = DEFAULT").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.ResetGlobalVariable("query_timeout"); err != nil {
		t.Fatalf("ResetGlobalVariable: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
