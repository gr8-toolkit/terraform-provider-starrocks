package starrocks

import (
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ---------------------------------------------------------------------------
// normaliseColumnType
// ---------------------------------------------------------------------------

func TestNormaliseColumnType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"bigint(20)", "BIGINT"},
		{"int(11)", "INT"},
		{"tinyint(4)", "TINYINT"},
		{"smallint(6)", "SMALLINT"},
		{"largeint(40)", "LARGEINT"},
		{"varchar(128)", "VARCHAR(128)"},     // VARCHAR keeps its length
		{"decimal(10, 2)", "DECIMAL(10, 2)"}, // DECIMAL keeps precision/scale
		{"BIGINT", "BIGINT"},                 // already canonical
		{"datetime", "DATETIME"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normaliseColumnType(tt.input); got != tt.want {
				t.Errorf("normaliseColumnType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildColumnDDL
// ---------------------------------------------------------------------------

func TestBuildColumnDDL(t *testing.T) {
	tests := []struct {
		name string
		col  ColumnDef
		want []string // all substrings that must appear in output
	}{
		{
			name: "simple not-null int",
			col:  ColumnDef{Name: "id", Type: "INT", Nullable: false},
			want: []string{"`id`", "INT", "NOT NULL"},
		},
		{
			name: "nullable varchar with default and comment",
			col: ColumnDef{
				Name:     "name",
				Type:     "VARCHAR(128)",
				Nullable: true,
				Default:  "anon",
				Comment:  "user name",
			},
			want: []string{"`name`", "VARCHAR(128)", "NULL", `DEFAULT "anon"`, `COMMENT "user name"`},
		},
		{
			name: "no default no comment",
			col:  ColumnDef{Name: "ts", Type: "DATETIME", Nullable: false},
			want: []string{"`ts`", "DATETIME", "NOT NULL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildColumnDDL(tt.col)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("buildColumnDDL(%+v) = %q, missing %q", tt.col, got, w)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseColumnLine
// ---------------------------------------------------------------------------

func TestParseColumnLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantCol ColumnDef
	}{
		{
			name:    "not-null int no default",
			line:    "  `id` int(11) NOT NULL COMMENT \"\",",
			wantOK:  true,
			wantCol: ColumnDef{Name: "id", Type: "INT", Nullable: false, Comment: ""},
		},
		{
			name:    "nullable varchar with default",
			line:    "  `name` varchar(128) NULL DEFAULT \"anon\" COMMENT \"user name\",",
			wantOK:  true,
			wantCol: ColumnDef{Name: "name", Type: "VARCHAR(128)", Nullable: true, Default: "anon", Comment: "user name"},
		},
		{
			name:    "bigint not null no comment",
			line:    "  `score` bigint(20) NOT NULL",
			wantOK:  true,
			wantCol: ColumnDef{Name: "score", Type: "BIGINT", Nullable: false},
		},
		{
			name:   "empty line skipped",
			line:   "",
			wantOK: false,
		},
		{
			name:   "index line skipped",
			line:   "  INDEX idx_name (`name`) USING BITMAP",
			wantOK: false,
		},
		{
			name:   "key line skipped",
			line:   "  PRIMARY KEY (`id`)",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseColumnLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseColumnLine(%q) ok=%v, want %v", tt.line, ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.Name != tt.wantCol.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantCol.Name)
			}
			if got.Type != tt.wantCol.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantCol.Type)
			}
			if got.Nullable != tt.wantCol.Nullable {
				t.Errorf("Nullable = %v, want %v", got.Nullable, tt.wantCol.Nullable)
			}
			if got.Default != tt.wantCol.Default {
				t.Errorf("Default = %q, want %q", got.Default, tt.wantCol.Default)
			}
			if got.Comment != tt.wantCol.Comment {
				t.Errorf("Comment = %q, want %q", got.Comment, tt.wantCol.Comment)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseCreateTable
// ---------------------------------------------------------------------------

func TestParseCreateTable(t *testing.T) {
	// This is the output StarRocks returns from SHOW CREATE TABLE for a simple
	// Duplicate Key table.
	ddl := "CREATE TABLE `events` (\n" +
		"  `id` bigint(20) NOT NULL COMMENT \"pk\",\n" +
		"  `name` varchar(128) NULL DEFAULT \"\" COMMENT \"\",\n" +
		"  `ts` datetime NOT NULL COMMENT \"\"\n" +
		") ENGINE=OLAP\n" +
		"DUPLICATE KEY(`id`)\n" +
		"COMMENT \"event log\"\n" +
		"DISTRIBUTED BY HASH(`id`) BUCKETS 4\n" +
		"PROPERTIES (\"replication_num\" = \"1\", \"in_memory\" = \"false\")"

	td, err := parseCreateTable("mydb", ddl)
	if err != nil {
		t.Fatalf("parseCreateTable: %v", err)
	}

	if td.Name != "events" {
		t.Errorf("Name = %q, want events", td.Name)
	}
	if td.Engine != "OLAP" {
		t.Errorf("Engine = %q, want OLAP", td.Engine)
	}
	if td.KeyType != "DUPLICATE KEY" {
		t.Errorf("KeyType = %q, want DUPLICATE KEY", td.KeyType)
	}
	if len(td.KeyColumns) != 1 || td.KeyColumns[0] != "id" {
		t.Errorf("KeyColumns = %v, want [id]", td.KeyColumns)
	}
	if len(td.Columns) != 3 {
		t.Fatalf("len(Columns) = %d, want 3", len(td.Columns))
	}

	// Check first column
	col := td.Columns[0]
	if col.Name != "id" {
		t.Errorf("Columns[0].Name = %q, want id", col.Name)
	}
	if col.Nullable {
		t.Error("Columns[0].Nullable = true, want false")
	}
	if col.Comment != "pk" {
		t.Errorf("Columns[0].Comment = %q, want pk", col.Comment)
	}

	// Check second column with default
	col2 := td.Columns[1]
	if col2.Name != "name" {
		t.Errorf("Columns[1].Name = %q, want name", col2.Name)
	}
	if !col2.Nullable {
		t.Error("Columns[1].Nullable = false, want true")
	}

	if !strings.Contains(td.DistBy, "DISTRIBUTED BY HASH") {
		t.Errorf("DistBy = %q, missing DISTRIBUTED BY HASH", td.DistBy)
	}
	if td.Properties["replication_num"] != "1" {
		t.Errorf("Properties[replication_num] = %q, want 1", td.Properties["replication_num"])
	}
}

func TestParseCreateTable_PrimaryKey(t *testing.T) {
	ddl := "CREATE TABLE `users` (\n" +
		"  `user_id` bigint(20) NOT NULL COMMENT \"\",\n" +
		"  `email` varchar(255) NULL COMMENT \"\"\n" +
		") ENGINE=OLAP\n" +
		"PRIMARY KEY(`user_id`)\n" +
		"DISTRIBUTED BY HASH(`user_id`)\n" +
		"PROPERTIES (\"replication_num\" = \"1\")"

	td, err := parseCreateTable("mydb", ddl)
	if err != nil {
		t.Fatalf("parseCreateTable: %v", err)
	}
	if td.KeyType != "PRIMARY KEY" {
		t.Errorf("KeyType = %q, want PRIMARY KEY", td.KeyType)
	}
	if len(td.Columns) != 2 {
		t.Errorf("len(Columns) = %d, want 2", len(td.Columns))
	}
}

// ---------------------------------------------------------------------------
// columnChanged
// ---------------------------------------------------------------------------

func TestColumnChanged(t *testing.T) {
	base := ColumnDef{Name: "x", Type: "INT", Nullable: true}

	tests := []struct {
		name    string
		old     ColumnDef
		new     ColumnDef
		changed bool
	}{
		{"identical", base, base, false},
		{"type changed", base, ColumnDef{Name: "x", Type: "BIGINT", Nullable: true}, true},
		{"nullable changed", base, ColumnDef{Name: "x", Type: "INT", Nullable: false}, true},
		{"default added", base, ColumnDef{Name: "x", Type: "INT", Nullable: true, Default: "0"}, true},
		{"comment added", base, ColumnDef{Name: "x", Type: "INT", Nullable: true, Comment: "note"}, true},
		{"type case insensitive same", base, ColumnDef{Name: "x", Type: "int", Nullable: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnChanged(tt.old, tt.new); got != tt.changed {
				t.Errorf("columnChanged = %v, want %v", got, tt.changed)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client — CreateTable (SQL shape)
// ---------------------------------------------------------------------------

func TestCreateTable_sqlShape(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))

	td := &TableDef{
		Name:       "events",
		Engine:     "OLAP",
		KeyType:    "DUPLICATE KEY",
		KeyColumns: []string{"id"},
		Columns: []ColumnDef{
			{Name: "id", Type: "BIGINT", Nullable: false},
			{Name: "name", Type: "VARCHAR(128)", Nullable: true},
		},
		DistBy:     "DISTRIBUTED BY HASH(`id`)",
		Properties: map[string]string{"replication_num": "1"},
	}

	if err := client.CreateTable("mydb", td); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Client — AlterTable operations
// ---------------------------------------------------------------------------

func TestAlterTableAddColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER TABLE `mydb`.`events` ADD COLUMN `score` INT NOT NULL").
		WillReturnResult(sqlmock.NewResult(0, 0))

	col := ColumnDef{Name: "score", Type: "INT", Nullable: false}
	if err := client.AlterTableAddColumn("mydb", "events", col); err != nil {
		t.Fatalf("AlterTableAddColumn: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAlterTableDropColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER TABLE `mydb`.`events` DROP COLUMN `score`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.AlterTableDropColumn("mydb", "events", "score"); err != nil {
		t.Fatalf("AlterTableDropColumn: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAlterTableModifyColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("ALTER TABLE `mydb`.`events` MODIFY COLUMN `score` BIGINT NOT NULL").
		WillReturnResult(sqlmock.NewResult(0, 0))

	col := ColumnDef{Name: "score", Type: "BIGINT", Nullable: false}
	if err := client.AlterTableModifyColumn("mydb", "events", col); err != nil {
		t.Fatalf("AlterTableModifyColumn: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Client — GetTable
// ---------------------------------------------------------------------------

func TestGetTable_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	// When the table doesn't exist StarRocks returns an error, not an empty
	// result set, but we also handle the nil-row case.
	mock.ExpectQuery("SHOW CREATE TABLE").WillReturnRows(
		sqlmock.NewRows([]string{"Table", "Create Table"}),
	)

	got, err := client.GetTable("mydb", "missing")
	if err != nil {
		t.Fatalf("GetTable returned unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetTable = %+v, want nil", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetTable_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	ddl := "CREATE TABLE `events` (\n" +
		"  `id` bigint(20) NOT NULL COMMENT \"\",\n" +
		"  `ts` datetime NOT NULL COMMENT \"\"\n" +
		") ENGINE=OLAP\n" +
		"DUPLICATE KEY(`id`)\n" +
		"DISTRIBUTED BY HASH(`id`)\n" +
		"PROPERTIES (\"replication_num\" = \"1\")"

	mock.ExpectQuery("SHOW CREATE TABLE `mydb`.`events`").WillReturnRows(
		sqlmock.NewRows([]string{"Table", "Create Table"}).AddRow("events", ddl),
	)

	got, err := client.GetTable("mydb", "events")
	if err != nil {
		t.Fatalf("GetTable: %v", err)
	}
	if got == nil {
		t.Fatal("GetTable returned nil, want table")
	}
	if got.Name != "events" {
		t.Errorf("Name = %q, want events", got.Name)
	}
	if len(got.Columns) != 2 {
		t.Errorf("len(Columns) = %d, want 2", len(got.Columns))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Client — DropTable
// ---------------------------------------------------------------------------

func TestDropTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &Client{db: db}

	mock.ExpectExec("DROP TABLE IF EXISTS `mydb`.`events`").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := client.DropTable("mydb", "events"); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
