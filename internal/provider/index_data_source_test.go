package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAcc_IndexDataSource_bitmap creates a BITMAP index via the managed
// resource, reads it back via the data source, and cross-checks all fields.
func TestAcc_IndexDataSource_bitmap(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() +
					accIndexTableConfig("idx_ds_bitmap_tbl") +
					accBitmapIndexConfig("idx_ds_bitmap_tbl", "idx_ds_payload", "payload") +
					accIndexDataSourceConfig(accDB, "idx_ds_bitmap_tbl", "idx_ds_payload"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_index.test", "database", accDB),
					resource.TestCheckResourceAttr("data.starrocks_index.test", "table", "idx_ds_bitmap_tbl"),
					resource.TestCheckResourceAttr("data.starrocks_index.test", "name", "idx_ds_payload"),
					resource.TestCheckResourceAttr("data.starrocks_index.test", "type", "BITMAP"),
					resource.TestCheckResourceAttr("data.starrocks_index.test", "column", "payload"),
					// Cross-check with the managed resource.
					resource.TestCheckResourceAttrPair(
						"data.starrocks_index.test", "type",
						"starrocks_index.test", "type",
					),
					resource.TestCheckResourceAttrPair(
						"data.starrocks_index.test", "column",
						"starrocks_index.test", "column",
					),
				),
			},
		},
	})
}

// TestAcc_IndexDataSource_withComment verifies that the data source correctly
// reads back the index comment.
func TestAcc_IndexDataSource_withComment(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() +
					accIndexTableConfig("idx_ds_comment_tbl") +
					accBitmapIndexWithCommentConfig("idx_ds_comment_tbl", "idx_ds_id", "id", "the id index") +
					accIndexDataSourceConfig(accDB, "idx_ds_comment_tbl", "idx_ds_id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_index.test", "comment", "the id index"),
					resource.TestCheckResourceAttrPair(
						"data.starrocks_index.test", "comment",
						"starrocks_index.test", "comment",
					),
				),
			},
		},
	})
}

// TestAcc_IndexDataSource_notFound verifies that looking up a non-existent
// index produces a clear error.
func TestAcc_IndexDataSource_notFound(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() +
					accIndexTableConfig("idx_ds_notfound_tbl") +
					accIndexDataSourceOnlyConfig(accDB, "idx_ds_notfound_tbl", "nonexistent_idx"),
				ExpectError: regexp.MustCompile(`Index not found`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accIndexDataSourceConfig returns a data source block that reads back an
// index created by an accompanying starrocks_index resource.
func accIndexDataSourceConfig(db, table, indexName string) string {
	return fmt.Sprintf(`
data "starrocks_index" "test" {
  database = %q
  table    = %q
  name     = starrocks_index.test.name
}
`, db, table)
}

// accIndexDataSourceOnlyConfig returns a standalone data source with no
// managing resource — used to test the not-found error path. The table must
// already exist (created via accIndexTableConfig).
func accIndexDataSourceOnlyConfig(db, table, indexName string) string {
	return fmt.Sprintf(`
data "starrocks_index" "test" {
  database = %q
  table    = %q
  name     = %q
}
`, db, table, indexName)
}
