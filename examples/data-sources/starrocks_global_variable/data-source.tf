data "starrocks_global_variable" "query_timeout" {
  name = "query_timeout"
}

output "current_query_timeout" {
  value = data.starrocks_global_variable.query_timeout.value
}
