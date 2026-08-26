package starrocks

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ---------------------------------------------------------------------------
// CreateCatalog
// ---------------------------------------------------------------------------

func TestCreateCatalog_hive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec(`CREATE EXTERNAL CATALOG`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.CreateCatalog("hive_catalog", "", map[string]string{
		"type":                "hive",
		"hive.metastore.uris": "thrift://meta:9083",
	}); err != nil {
		t.Fatalf("CreateCatalog: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateCatalog_withComment(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec(`CREATE EXTERNAL CATALOG`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.CreateCatalog("iceberg_cat", "My Iceberg catalog", map[string]string{
		"type":                 "iceberg",
		"iceberg.catalog.type": "hive",
	}); err != nil {
		t.Fatalf("CreateCatalog with comment: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCreateCatalog_sqlShape verifies the SQL text produced by the builder
// contains all expected fragments, without a live DB.
func TestCreateCatalog_sqlShape(t *testing.T) {
	props := map[string]string{
		"type":                "hive",
		"hive.metastore.uris": "thrift://meta:9083",
	}
	got := buildCreateCatalogSQL("my_hive", "a comment", props)

	for _, want := range []string{
		"CREATE EXTERNAL CATALOG `my_hive`",
		`COMMENT "a comment"`,
		"PROPERTIES",
		`"hive.metastore.uris" = "thrift://meta:9083"`,
		`"type" = "hive"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL missing %q\nSQL: %s", want, got)
		}
	}
}

// buildCreateCatalogSQL is the pure-Go SQL builder extracted for unit testing.
// It mirrors the logic inside Client.CreateCatalog exactly.
func buildCreateCatalogSQL(name, comment string, properties map[string]string) string {
	q := fmt.Sprintf("CREATE EXTERNAL CATALOG `%s`", name)
	if comment != "" {
		q += fmt.Sprintf(" COMMENT %q", comment)
	}
	if len(properties) > 0 {
		var pairs []string
		for k, v := range properties {
			pairs = append(pairs, fmt.Sprintf("%q = %q", k, v))
		}
		sort.Strings(pairs)
		q += " PROPERTIES (" + strings.Join(pairs, ", ") + ")"
	}
	return q
}

// ---------------------------------------------------------------------------
// GetCatalog
// ---------------------------------------------------------------------------

func TestGetCatalog_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	cols := []string{"Catalog", "Type", "Comment"}
	mock.ExpectQuery("SHOW CATALOGS LIKE 'hive_catalog'").WillReturnRows(
		sqlmock.NewRows(cols).AddRow("hive_catalog", "Hive", ""),
	)

	cat, err := client.GetCatalog("hive_catalog")
	if err != nil {
		t.Fatalf("GetCatalog: %v", err)
	}
	if cat == nil {
		t.Fatal("GetCatalog returned nil, want catalog")
	}
	if cat.Name != "hive_catalog" {
		t.Errorf("Name = %q, want %q", cat.Name, "hive_catalog")
	}
	if cat.Type != "Hive" {
		t.Errorf("Type = %q, want %q", cat.Type, "Hive")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetCatalog_internalCatalog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	cols := []string{"Catalog", "Type", "Comment"}
	mock.ExpectQuery("SHOW CATALOGS LIKE 'default_catalog'").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"default_catalog",
			"Internal",
			"An internal catalog contains this cluster's self-managed tables.",
		),
	)

	cat, err := client.GetCatalog("default_catalog")
	if err != nil {
		t.Fatalf("GetCatalog: %v", err)
	}
	if cat == nil {
		t.Fatal("GetCatalog returned nil for default_catalog")
	}
	if cat.Type != "Internal" {
		t.Errorf("Type = %q, want Internal", cat.Type)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetCatalog_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	cols := []string{"Catalog", "Type", "Comment"}
	// SHOW CATALOGS LIKE returns zero rows when no catalog matches.
	mock.ExpectQuery("SHOW CATALOGS LIKE 'missing_cat'").WillReturnRows(
		sqlmock.NewRows(cols),
	)

	cat, err := client.GetCatalog("missing_cat")
	if err != nil {
		t.Fatalf("GetCatalog returned unexpected error: %v", err)
	}
	if cat != nil {
		t.Errorf("GetCatalog = %+v, want nil", cat)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteCatalog
// ---------------------------------------------------------------------------

func TestDeleteCatalog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("DROP CATALOG IF EXISTS `hive_catalog`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.DeleteCatalog("hive_catalog"); err != nil {
		t.Fatalf("DeleteCatalog: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
