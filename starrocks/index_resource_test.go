package starrocks

import (
	"fmt"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ---------------------------------------------------------------------------
// CreateIndex — SQL shape
// ---------------------------------------------------------------------------

func TestCreateIndex_bitmap(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// Expect the ALTER TABLE ADD INDEX statement.
	mock.ExpectExec("ALTER TABLE `mydb`.`events` ADD INDEX `idx_name` \\(`name`\\) USING BITMAP").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect the SHOW ALTER TABLE COLUMN poll that returns FINISHED immediately.
	mock.ExpectQuery("SHOW ALTER TABLE COLUMN FROM `mydb`").WillReturnRows(
		sqlmock.NewRows([]string{"JobId", "TableName", "State"}).
			AddRow("1", "events", "FINISHED"),
	)

	// Suppress real sleeps in tests.
	orig := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = orig }()

	idx := IndexDef{Name: "idx_name", Column: "name", Type: "BITMAP"}
	if err := client.CreateIndex("mydb", "events", idx, 10); err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateIndex_ngrambf(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER TABLE `mydb`.`t1` ADD INDEX `idx_desc`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SHOW ALTER TABLE COLUMN FROM `mydb`").WillReturnRows(
		sqlmock.NewRows([]string{"JobId", "TableName", "State"}).AddRow("2", "t1", "FINISHED"),
	)

	orig := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = orig }()

	idx := IndexDef{
		Name:   "idx_desc",
		Column: "description",
		Type:   "NGRAMBF",
		Properties: map[string]string{
			"gram_num":         "4",
			"bloom_filter_fpp": "0.05",
		},
	}
	if err := client.CreateIndex("mydb", "t1", idx, 10); err != nil {
		t.Fatalf("CreateIndex NGRAMBF: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateIndex_withComment(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SHOW ALTER TABLE COLUMN").WillReturnRows(
		sqlmock.NewRows([]string{"JobId", "TableName", "State"}).AddRow("3", "events", "FINISHED"),
	)

	orig := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = orig }()

	idx := IndexDef{Name: "idx_id", Column: "id", Type: "BITMAP", Comment: "primary key index"}
	if err := client.CreateIndex("mydb", "events", idx, 10); err != nil {
		t.Fatalf("CreateIndex with comment: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateIndex — job polling: CANCELLED returns an error
// ---------------------------------------------------------------------------

func TestCreateIndex_jobCancelled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SHOW ALTER TABLE COLUMN").WillReturnRows(
		sqlmock.NewRows([]string{"JobId", "TableName", "State"}).AddRow("4", "events", "CANCELLED"),
	)

	orig := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = orig }()

	idx := IndexDef{Name: "idx_id", Column: "id", Type: "BITMAP"}
	err = client.CreateIndex("mydb", "events", idx, 10)
	if err == nil {
		t.Fatal("expected error for CANCELLED job, got nil")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error = %q, want 'cancelled'", err.Error())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetIndex
// ---------------------------------------------------------------------------

func TestGetIndex_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	cols := []string{
		"Table", "Non_unique", "Key_name", "Seq_in_index",
		"Column_name", "Collation", "Cardinality", "Sub_part",
		"Packed", "Null", "Index_type", "Comment",
	}
	mock.ExpectQuery("SHOW INDEXES FROM `mydb`.`events`").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"mydb.events", "", "idx_name", "",
			"name", "", "", "",
			"", "", "BITMAP", "my comment",
		),
	)

	got, err := client.GetIndex("mydb", "events", "idx_name")
	if err != nil {
		t.Fatalf("GetIndex: %v", err)
	}
	if got == nil {
		t.Fatal("GetIndex returned nil, want index")
	}
	if got.Name != "idx_name" {
		t.Errorf("Name = %q, want idx_name", got.Name)
	}
	if got.Column != "name" {
		t.Errorf("Column = %q, want name", got.Column)
	}
	if got.Type != "BITMAP" {
		t.Errorf("Type = %q, want BITMAP", got.Type)
	}
	if got.Comment != "my comment" {
		t.Errorf("Comment = %q, want 'my comment'", got.Comment)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetIndex_ngrambfTypeStripped verifies that the resolved-properties suffix
// StarRocks appends to the Index_type column for NGRAMBF/GIN/VECTOR is stripped,
// so the returned type matches what the user wrote in config.
func TestGetIndex_ngrambfTypeStripped(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	cols := []string{
		"Table", "Non_unique", "Key_name", "Seq_in_index",
		"Column_name", "Collation", "Cardinality", "Sub_part",
		"Packed", "Null", "Index_type", "Comment",
	}
	// StarRocks returns the full resolved properties in the Index_type column.
	rawType := `NGRAMBF("BLOOM_FILTER_FPP" = "0.05", "CASE_SENSITIVE" = "TRUE", "GRAM_NUM" = "4")`
	mock.ExpectQuery("SHOW INDEXES FROM `mydb`.`t1`").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"mydb.t1", "", "idx_desc", "",
			"description", "", "", "",
			"", "", rawType, "",
		),
	)

	got, err := client.GetIndex("mydb", "t1", "idx_desc")
	if err != nil {
		t.Fatalf("GetIndex: %v", err)
	}
	if got == nil {
		t.Fatal("GetIndex returned nil")
	}
	if got.Type != "NGRAMBF" {
		t.Errorf("Type = %q, want NGRAMBF (suffix should be stripped)", got.Type)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetIndex_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	cols := []string{"Table", "Non_unique", "Key_name", "Seq_in_index",
		"Column_name", "Collation", "Cardinality", "Sub_part",
		"Packed", "Null", "Index_type", "Comment"}
	// Return a different index name — GetIndex should return nil.
	mock.ExpectQuery("SHOW INDEXES FROM `mydb`.`events`").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"mydb.events", "", "other_idx", "",
			"id", "", "", "",
			"", "", "BITMAP", "",
		),
	)

	got, err := client.GetIndex("mydb", "events", "idx_name")
	if err != nil {
		t.Fatalf("GetIndex returned unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetIndex = %+v, want nil", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetIndex_emptyTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	cols := []string{"Table", "Non_unique", "Key_name", "Seq_in_index",
		"Column_name", "Collation", "Cardinality", "Sub_part",
		"Packed", "Null", "Index_type", "Comment"}
	mock.ExpectQuery("SHOW INDEXES FROM `mydb`.`events`").WillReturnRows(
		sqlmock.NewRows(cols), // zero rows
	)

	got, err := client.GetIndex("mydb", "events", "idx_name")
	if err != nil {
		t.Fatalf("GetIndex: %v", err)
	}
	if got != nil {
		t.Errorf("GetIndex = %+v, want nil", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DropIndex
// ---------------------------------------------------------------------------

func TestDropIndex(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("DROP INDEX `idx_name` ON `mydb`.`events`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SHOW ALTER TABLE COLUMN FROM `mydb`").WillReturnRows(
		sqlmock.NewRows([]string{"JobId", "TableName", "State"}).AddRow("5", "events", "FINISHED"),
	)

	orig := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = orig }()

	if err := client.DropIndex("mydb", "events", "idx_name", 10); err != nil {
		t.Fatalf("DropIndex: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// isIndexNotFoundError
// ---------------------------------------------------------------------------

func TestIsIndexNotFoundError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"Error 1064: Index idx_name does not exist", true},
		{"index idx_name is not found", true},
		{"index not found", true},
		{"Error 1064: Table events is not found", true}, // table gone → index also gone
		{"some other error", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := fmt.Errorf("%s", tt.msg)
			if got := isIndexNotFoundError(err); got != tt.want {
				t.Errorf("isIndexNotFoundError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}
