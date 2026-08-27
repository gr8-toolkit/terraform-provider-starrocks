data "starrocks_table" "example" {
  database = "my_database"
  name     = "my_table"
}

output "table_engine" {
  value = data.starrocks_table.example.engine
}

output "table_columns" {
  value = data.starrocks_table.example.columns
}

output "table_key_type" {
  value = data.starrocks_table.example.key_type
}
