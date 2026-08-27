package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// All index acceptance tests create their own table inside accDB (tf_acc_test),
// which is created and torn down by TestMain.
//
// Index creation in StarRocks is asynchronous. The provider's timeout attribute
// is set to 120 s — more than enough for the all-in-one Docker image.

// TestAcc_Index_bitmap creates a BITMAP index, checks computed fields, and
// verifies that import by "database.table.index_name" round-trips cleanly.
func TestAcc_Index_bitmap(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accIndexTableConfig("idx_bitmap_tbl") +
					accBitmapIndexConfig("idx_bitmap_tbl", "idx_payload", "payload"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_index.test", "database", accDB),
					resource.TestCheckResourceAttr("starrocks_index.test", "table", "idx_bitmap_tbl"),
					resource.TestCheckResourceAttr("starrocks_index.test", "name", "idx_payload"),
					resource.TestCheckResourceAttr("starrocks_index.test", "type", "BITMAP"),
					resource.TestCheckResourceAttr("starrocks_index.test", "column", "payload"),
				),
			},
			// Import by "database.table.index_name".
			{
				ResourceName:      "starrocks_index.test",
				ImportState:       true,
				ImportStateId:     accDB + ".idx_bitmap_tbl.idx_payload",
				ImportStateVerify: false, // properties not returned by SHOW INDEXES
			},
		},
	})
}

// TestAcc_Index_withComment creates a BITMAP index with a comment and checks
// it survives the create/read round-trip.
func TestAcc_Index_withComment(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accIndexTableConfig("idx_comment_tbl") +
					accBitmapIndexWithCommentConfig("idx_comment_tbl", "idx_id", "id", "the id index"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_index.test", "name", "idx_id"),
					resource.TestCheckResourceAttr("starrocks_index.test", "comment", "the id index"),
				),
			},
		},
	})
}

// TestAcc_Index_ngrambf creates an NGRAMBF index on a VARCHAR column.
func TestAcc_Index_ngrambf(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accIndexTableConfig("idx_ngram_tbl") +
					accNgramIndexConfig("idx_ngram_tbl", "idx_payload_ngram", "payload"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_index.test", "type", "NGRAMBF"),
					resource.TestCheckResourceAttr("starrocks_index.test", "column", "payload"),
				),
			},
		},
	})
}

// TestAcc_Index_disappears verifies that when the index is dropped outside
// Terraform, Read removes it from state and the next plan proposes a re-create.
func TestAcc_Index_disappears(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accIndexTableConfig("idx_disappears_tbl") +
					accBitmapIndexConfig("idx_disappears_tbl", "idx_disappears", "payload"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_index.test", "name", "idx_disappears"),
					testAccDropIndexOutOfBand(accDB, "idx_disappears_tbl", "idx_disappears"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accIndexTableConfig creates a starrocks_table resource that index tests
// build their indexes on. Each test uses a unique table name to avoid
// cross-test interference when tests run in parallel or share state.
func accIndexTableConfig(tableName string) string {
	return fmt.Sprintf(`
resource "starrocks_table" "base" {
  database = %q
  name     = %q

  key_type    = "DUPLICATE KEY"
  key_columns = ["id"]

  columns = [
    {
      name     = "id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "payload"
      type     = "VARCHAR(256)"
      nullable = true
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(id) BUCKETS 1"

  properties = {
    "replication_num" = "1"
  }
}
`, accDB, tableName)
}

// accBitmapIndexConfig returns HCL for a BITMAP index that depends on the
// starrocks_table.base resource created by accIndexTableConfig.
func accBitmapIndexConfig(tableName, indexName, column string) string {
	return fmt.Sprintf(`
resource "starrocks_index" "test" {
  database = %q
  table    = starrocks_table.base.name
  name     = %q
  type     = "BITMAP"
  column   = %q
  timeout  = 120
}
`, accDB, indexName, column)
}

// accBitmapIndexWithCommentConfig is like accBitmapIndexConfig but adds a comment.
func accBitmapIndexWithCommentConfig(tableName, indexName, column, comment string) string {
	return fmt.Sprintf(`
resource "starrocks_index" "test" {
  database = %q
  table    = starrocks_table.base.name
  name     = %q
  type     = "BITMAP"
  column   = %q
  comment  = %q
  timeout  = 120
}
`, accDB, indexName, column, comment)
}

// accNgramIndexConfig returns HCL for an NGRAMBF index with gram_num=4.
func accNgramIndexConfig(tableName, indexName, column string) string {
	return fmt.Sprintf(`
resource "starrocks_index" "test" {
  database = %q
  table    = starrocks_table.base.name
  name     = %q
  type     = "NGRAMBF"
  column   = %q
  timeout  = 120

  properties = {
    "gram_num"        = "4"
    "bloom_filter_fpp" = "0.05"
  }
}
`, accDB, indexName, column)
}

// ---------------------------------------------------------------------------
// Check helpers
// ---------------------------------------------------------------------------

func testAccDropIndexOutOfBand(db, table, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client for out-of-band index drop: %w", err)
		}
		return client.DropIndex(db, table, name, 120)
	}
}
