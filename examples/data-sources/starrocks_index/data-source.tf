data "starrocks_index" "example" {
  database = "my_database"
  table    = "my_table"
  name     = "idx_user_id"
}

output "index_type" {
  value = data.starrocks_index.example.type
}

output "index_column" {
  value = data.starrocks_index.example.column
}
