package client

import (
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// pluginCols lists the columns returned by SHOW PLUGINS in the order StarRocks
// emits them. The dynamic column scan in GetPlugin is robust against any order,
// but using a realistic set makes the tests more representative.
var pluginCols = []string{
	"Name", "Type", "Description", "Version",
	"JavaVersion", "ClassName", "SoName", "Sources", "Status", "Properties",
}

// ---------------------------------------------------------------------------
// InstallPlugin
// ---------------------------------------------------------------------------

func TestInstallPlugin_simple(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectExec(`INSTALL PLUGIN FROM`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := c.InstallPlugin("/path/to/plugin.zip", nil); err != nil {
		t.Fatalf("InstallPlugin: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestInstallPlugin_withProperties(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectExec(`INSTALL PLUGIN FROM`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := c.InstallPlugin("http://example.com/plugin.zip", map[string]string{
		"md5sum": "73877f6029216f4314d712086a146570",
	}); err != nil {
		t.Fatalf("InstallPlugin with properties: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestInstallPlugin_sqlShape verifies the exact SQL fragments for a
// multi-property install, including deterministic property ordering.
func TestInstallPlugin_sqlShape(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
		func(_, actual string) error {
			for _, want := range []string{
				`INSTALL PLUGIN FROM "http://example.com/plugin.zip"`,
				"PROPERTIES",
				`"md5sum" = "abc123"`,
			} {
				if !strings.Contains(actual, want) {
					t.Errorf("SQL missing %q\nSQL: %s", want, actual)
				}
			}
			return nil
		},
	)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 0))

	c := &Client{DB: db}
	_ = c.InstallPlugin("http://example.com/plugin.zip", map[string]string{"md5sum": "abc123"})
}

// ---------------------------------------------------------------------------
// GetPlugin
// ---------------------------------------------------------------------------

func TestGetPlugin_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectQuery("SHOW PLUGINS").WillReturnRows(
		sqlmock.NewRows(pluginCols).AddRow(
			"auditdemo", "AUDIT", "An audit plugin", "0.12",
			"1.8.0", "com.starrocks.AuditPlugin", "libaudit.so",
			"/path/to/auditdemo.zip", "INSTALLED", "{}",
		),
	)

	p, err := c.GetPlugin("auditdemo")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	if p == nil {
		t.Fatal("GetPlugin returned nil, want plugin")
	}
	if p.Name != "auditdemo" {
		t.Errorf("Name = %q, want auditdemo", p.Name)
	}
	if p.Type != "AUDIT" {
		t.Errorf("Type = %q, want AUDIT", p.Type)
	}
	if p.Status != "INSTALLED" {
		t.Errorf("Status = %q, want INSTALLED", p.Status)
	}
	if p.Description != "An audit plugin" {
		t.Errorf("Description = %q, want 'An audit plugin'", p.Description)
	}
	if p.Version != "0.12" {
		t.Errorf("Version = %q, want 0.12", p.Version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetPlugin_caseInsensitive verifies that name lookup is case-insensitive.
func TestGetPlugin_caseInsensitive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectQuery("SHOW PLUGINS").WillReturnRows(
		sqlmock.NewRows(pluginCols).AddRow(
			"AuditDemo", "AUDIT", "", "1.0",
			"", "", "", "", "INSTALLED", "",
		),
	)

	// Request with different casing — should still match.
	p, err := c.GetPlugin("auditdemo")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	if p == nil {
		t.Fatal("GetPlugin returned nil for case-insensitive match")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetPlugin_notFound verifies that zero matching rows returns nil without
// an error.
func TestGetPlugin_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectQuery("SHOW PLUGINS").WillReturnRows(
		sqlmock.NewRows(pluginCols),
	)

	p, err := c.GetPlugin("missing_plugin")
	if err != nil {
		t.Fatalf("GetPlugin returned unexpected error: %v", err)
	}
	if p != nil {
		t.Errorf("GetPlugin = %+v, want nil", p)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetPlugin_multipleRows verifies that GetPlugin returns only the matching
// plugin when SHOW PLUGINS returns multiple rows.
func TestGetPlugin_multipleRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectQuery("SHOW PLUGINS").WillReturnRows(
		sqlmock.NewRows(pluginCols).
			AddRow("other_plugin", "STORAGE", "", "2.0", "", "", "", "", "INSTALLED", "").
			AddRow("auditdemo", "AUDIT", "My audit plugin", "0.12", "", "", "", "", "INSTALLED", "").
			AddRow("another_plugin", "AUDIT", "", "1.0", "", "", "", "", "INSTALLED", ""),
	)

	p, err := c.GetPlugin("auditdemo")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	if p == nil {
		t.Fatal("GetPlugin returned nil, want auditdemo")
	}
	if p.Name != "auditdemo" {
		t.Errorf("Name = %q, want auditdemo", p.Name)
	}
	if p.Type != "AUDIT" {
		t.Errorf("Type = %q, want AUDIT", p.Type)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// UninstallPlugin
// ---------------------------------------------------------------------------

func TestUninstallPlugin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectExec("UNINSTALL PLUGIN auditdemo").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := c.UninstallPlugin("auditdemo"); err != nil {
		t.Fatalf("UninstallPlugin: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
