data "starrocks_resource_group" "example" {
  name = "my_resource_group"
}

output "mem_limit" {
  value = data.starrocks_resource_group.example.mem_limit
}

output "concurrency_limit" {
  value = data.starrocks_resource_group.example.concurrency_limit
}
