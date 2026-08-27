package client

import (
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// resourceGroupModelStub — minimal ResourceGroupModel for client-layer tests
// ---------------------------------------------------------------------------

type resourceGroupModelStub struct {
	name                   types.String
	cpuWeight              types.Int64
	exclusiveCPUCores      types.Int64
	cpuCoreLimit           types.Int64
	maxCPUCores            types.Int64
	memLimit               types.String
	concurrencyLimit       types.Int64
	bigQueryMemLimit       types.Int64
	bigQueryScanRowsLimit  types.Int64
	bigQueryCPUSecondLimit types.Int64
	classifiers            types.List
}

func (m *resourceGroupModelStub) GetName() types.String             { return m.name }
func (m *resourceGroupModelStub) GetCPUWeight() types.Int64         { return m.cpuWeight }
func (m *resourceGroupModelStub) GetExclusiveCPUCores() types.Int64 { return m.exclusiveCPUCores }
func (m *resourceGroupModelStub) GetCPUCoreLimit() types.Int64      { return m.cpuCoreLimit }
func (m *resourceGroupModelStub) GetMaxCPUCores() types.Int64       { return m.maxCPUCores }
func (m *resourceGroupModelStub) GetMemLimit() types.String         { return m.memLimit }
func (m *resourceGroupModelStub) GetConcurrencyLimit() types.Int64  { return m.concurrencyLimit }
func (m *resourceGroupModelStub) GetBigQueryMemLimit() types.Int64  { return m.bigQueryMemLimit }
func (m *resourceGroupModelStub) GetBigQueryScanRowsLimit() types.Int64 {
	return m.bigQueryScanRowsLimit
}
func (m *resourceGroupModelStub) GetBigQueryCPUSecondLimit() types.Int64 {
	return m.bigQueryCPUSecondLimit
}
func (m *resourceGroupModelStub) GetClassifiers() types.List { return m.classifiers }

// ---------------------------------------------------------------------------
// CreateResourceGroup
// ---------------------------------------------------------------------------

func TestCreateResourceGroup_minimal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectExec("CREATE RESOURCE GROUP `rg_min`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	m := &resourceGroupModelStub{
		name:        types.StringValue("rg_min"),
		classifiers: types.ListValueMust(types.StringType, nil),
	}
	if err := c.CreateResourceGroup(m); err != nil {
		t.Fatalf("CreateResourceGroup: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateResourceGroup_withProperties(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectExec("CREATE RESOURCE GROUP `rg_props`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	m := &resourceGroupModelStub{
		name:             types.StringValue("rg_props"),
		cpuWeight:        types.Int64Value(4),
		memLimit:         types.StringValue("80.0%"),
		concurrencyLimit: types.Int64Value(10),
		classifiers:      types.ListValueMust(types.StringType, nil),
	}
	if err := c.CreateResourceGroup(m); err != nil {
		t.Fatalf("CreateResourceGroup: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCreateResourceGroup_sqlShape verifies the exact SQL fragments produced
// for a fully-populated model, without a live DB.
func TestCreateResourceGroup_sqlShape(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	// Capture the exact query string via a broad regex match, then inspect it.
	var capturedSQL string
	mock.ExpectExec("CREATE RESOURCE GROUP").
		WillReturnResult(sqlmock.NewResult(0, 0)).
		WillDelayFor(0)

	// Replace the exec with one that captures the query.
	// go-sqlmock matches by regex so we use a permissive pattern and inspect
	// the query via a custom matcher below.
	db2, mock2, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
		func(expectedSQL, actualSQL string) error {
			capturedSQL = actualSQL
			return nil
		},
	)))
	defer db2.Close()
	c2 := &Client{DB: db2}
	mock2.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 0))

	m := &resourceGroupModelStub{
		name:                   types.StringValue("rg_full"),
		cpuWeight:              types.Int64Value(2),
		memLimit:               types.StringValue("50.0%"),
		concurrencyLimit:       types.Int64Value(5),
		bigQueryMemLimit:       types.Int64Value(1073741824),
		bigQueryScanRowsLimit:  types.Int64Value(100000),
		bigQueryCPUSecondLimit: types.Int64Value(200),
		classifiers:            types.ListValueMust(types.StringType, nil),
	}
	_ = c.CreateResourceGroup(m) // fire against original mock to satisfy it
	_ = c2.CreateResourceGroup(m)

	for _, want := range []string{
		"CREATE RESOURCE GROUP `rg_full`",
		"'cpu_weight' = '2'",
		"'mem_limit' = '50.0%'",
		"'concurrency_limit' = '5'",
		"'big_query_mem_limit' = '1073741824'",
		"'big_query_scan_rows_limit' = '100000'",
		"'big_query_cpu_second_limit' = '200'",
	} {
		if !strings.Contains(capturedSQL, want) {
			t.Errorf("SQL missing %q\nSQL: %s", want, capturedSQL)
		}
	}
}

func TestCreateResourceGroup_nullClassifiers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectExec("CREATE RESOURCE GROUP `rg_nocls`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Null classifiers — no TO clause should be emitted.
	m := &resourceGroupModelStub{
		name:        types.StringValue("rg_nocls"),
		cpuWeight:   types.Int64Value(1),
		classifiers: types.ListNull(types.StringType),
	}
	if err := c.CreateResourceGroup(m); err != nil {
		t.Fatalf("CreateResourceGroup null classifiers: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetResourceGroup — column-count variants
// ---------------------------------------------------------------------------

func TestGetResourceGroup_12Columns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	// StarRocks 4.1.1 returns 12 columns including "warehouses".
	cols := []string{
		"name", "id", "cpu_weight", "exclusive_cpu_cores", "mem_limit",
		"big_query_cpu_second_limit", "big_query_scan_rows_limit", "big_query_mem_limit",
		"concurrency_limit", "spill_mem_limit_threshold", "warehouses", "classifiers",
	}
	mock.ExpectQuery("SHOW RESOURCE GROUP `test_rg`").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"test_rg", "1", "10", "0", "80.0%",
			"100", "500000", "1073741824",
			"20", "80%", "", "(id=1, user=test_user)",
		),
	)

	rg, err := c.GetResourceGroup("test_rg")
	if err != nil {
		t.Fatalf("GetResourceGroup: %v", err)
	}
	if rg == nil {
		t.Fatal("GetResourceGroup returned nil, want result")
	}
	if rg.Name.ValueString() != "test_rg" {
		t.Errorf("Name = %q, want test_rg", rg.Name.ValueString())
	}
	if rg.MemLimit.ValueString() != "80.0%" {
		t.Errorf("MemLimit = %q, want 80.0%%", rg.MemLimit.ValueString())
	}
	if rg.ConcurrencyLimit.ValueInt64() != 20 {
		t.Errorf("ConcurrencyLimit = %d, want 20", rg.ConcurrencyLimit.ValueInt64())
	}
	if rg.BigQueryMemLimit.ValueInt64() != 1073741824 {
		t.Errorf("BigQueryMemLimit = %d, want 1073741824", rg.BigQueryMemLimit.ValueInt64())
	}
	if rg.BigQueryScanRowsLimit.ValueInt64() != 500000 {
		t.Errorf("BigQueryScanRowsLimit = %d, want 500000", rg.BigQueryScanRowsLimit.ValueInt64())
	}
	if rg.BigQueryCPUSecondLimit.ValueInt64() != 100 {
		t.Errorf("BigQueryCPUSecondLimit = %d, want 100", rg.BigQueryCPUSecondLimit.ValueInt64())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetResourceGroup_11Columns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	// Older StarRocks without "warehouses" column.
	cols := []string{
		"name", "id", "cpu_weight", "exclusive_cpu_cores", "mem_limit",
		"big_query_cpu_second_limit", "big_query_scan_rows_limit", "big_query_mem_limit",
		"concurrency_limit", "spill_mem_limit_threshold", "classifiers",
	}
	mock.ExpectQuery("SHOW RESOURCE GROUP `test_rg`").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"test_rg", "1", "10", "0", "80.0%",
			"100", "500000", "1073741824",
			"20", "80%", "(id=1, user=test_user)",
		),
	)

	rg, err := c.GetResourceGroup("test_rg")
	if err != nil {
		t.Fatalf("GetResourceGroup: %v", err)
	}
	if rg.ConcurrencyLimit.ValueInt64() != 20 {
		t.Errorf("ConcurrencyLimit = %d, want 20", rg.ConcurrencyLimit.ValueInt64())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetResourceGroup_zeroRows verifies that an empty result set returns a
// non-nil ResourceGroup with just the name set (no properties populated).
// The client layer returns what it has; the caller (provider Read) decides
// whether to treat zero rows as "not found".
func TestGetResourceGroup_zeroRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	cols := []string{"name", "id", "mem_limit", "concurrency_limit", "classifiers"}
	mock.ExpectQuery("SHOW RESOURCE GROUP `missing`").WillReturnRows(
		sqlmock.NewRows(cols),
	)

	rg, err := c.GetResourceGroup("missing")
	if err != nil {
		t.Fatalf("GetResourceGroup: %v", err)
	}
	// Zero rows → struct initialised with just the name; all numeric fields null.
	if rg == nil {
		t.Fatal("GetResourceGroup returned nil, want non-nil")
	}
	if rg.Name.ValueString() != "missing" {
		t.Errorf("Name = %q, want missing", rg.Name.ValueString())
	}
	if !rg.MemLimit.IsNull() {
		t.Errorf("MemLimit should be null for zero-row result, got %q", rg.MemLimit.ValueString())
	}
	if !rg.ConcurrencyLimit.IsNull() {
		t.Errorf("ConcurrencyLimit should be null for zero-row result, got %d", rg.ConcurrencyLimit.ValueInt64())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetResourceGroup_zeroValueFields verifies that numeric columns with
// value "0" are treated as unset (null) rather than zero — matching StarRocks
// semantics where 0 means "not configured".
func TestGetResourceGroup_zeroValueFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	cols := []string{
		"name", "id", "cpu_weight", "exclusive_cpu_cores", "mem_limit",
		"big_query_cpu_second_limit", "big_query_scan_rows_limit", "big_query_mem_limit",
		"concurrency_limit", "spill_mem_limit_threshold", "classifiers",
	}
	// All numeric fields set to "0" — none should be populated in the result.
	mock.ExpectQuery("SHOW RESOURCE GROUP `rg_zeros`").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"rg_zeros", "1", "0", "0", "80.0%",
			"0", "0", "0",
			"0", "0", "",
		),
	)

	rg, err := c.GetResourceGroup("rg_zeros")
	if err != nil {
		t.Fatalf("GetResourceGroup: %v", err)
	}
	if !rg.ConcurrencyLimit.IsNull() {
		t.Errorf("ConcurrencyLimit should be null for value 0, got %d", rg.ConcurrencyLimit.ValueInt64())
	}
	if !rg.BigQueryMemLimit.IsNull() {
		t.Errorf("BigQueryMemLimit should be null for value 0, got %d", rg.BigQueryMemLimit.ValueInt64())
	}
	if !rg.BigQueryScanRowsLimit.IsNull() {
		t.Errorf("BigQueryScanRowsLimit should be null for value 0, got %d", rg.BigQueryScanRowsLimit.ValueInt64())
	}
	if !rg.BigQueryCPUSecondLimit.IsNull() {
		t.Errorf("BigQueryCPUSecondLimit should be null for value 0, got %d", rg.BigQueryCPUSecondLimit.ValueInt64())
	}
	// mem_limit is a string — it should still be populated.
	if rg.MemLimit.ValueString() != "80.0%" {
		t.Errorf("MemLimit = %q, want 80.0%%", rg.MemLimit.ValueString())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteResourceGroup
// ---------------------------------------------------------------------------

func TestDeleteResourceGroup(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &Client{DB: db}

	mock.ExpectExec("DROP RESOURCE GROUP `rg_to_delete`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := c.DeleteResourceGroup("rg_to_delete"); err != nil {
		t.Fatalf("DeleteResourceGroup: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseClassifier
// ---------------------------------------------------------------------------

func TestParseClassifier(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    int64
		wantUser  string
		wantRole  string
		wantQT    string
		wantSrcIP string
		wantDB    string
	}{
		{
			name:     "user only",
			input:    "id=1, user=alice",
			wantID:   1,
			wantUser: "alice",
		},
		{
			name:   "id without user — regex requires user field, ID not parsed",
			input:  "id=42",
			wantID: 0, // parseClassifier regex requires user= after id=; bare id is not matched
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseClassifier(tt.input)
			if got.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", got.ID, tt.wantID)
			}
			if tt.wantUser != "" && got.User.ValueString() != tt.wantUser {
				t.Errorf("User = %q, want %q", got.User.ValueString(), tt.wantUser)
			}
			if tt.wantRole != "" && got.Role.ValueString() != tt.wantRole {
				t.Errorf("Role = %q, want %q", got.Role.ValueString(), tt.wantRole)
			}
			if tt.wantQT != "" && got.QueryType.ValueString() != tt.wantQT {
				t.Errorf("QueryType = %q, want %q", got.QueryType.ValueString(), tt.wantQT)
			}
		})
	}
}
