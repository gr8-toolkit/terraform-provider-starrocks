data "starrocks_database" "example" {
  name = "my_database"
}

output "database_exists" {
  value = data.starrocks_database.example.exists
}
