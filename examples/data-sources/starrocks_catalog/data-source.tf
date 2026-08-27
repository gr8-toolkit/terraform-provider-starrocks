data "starrocks_catalog" "default" {
  name = "default_catalog"
}

output "catalog_type" {
  value = data.starrocks_catalog.default.type
}
