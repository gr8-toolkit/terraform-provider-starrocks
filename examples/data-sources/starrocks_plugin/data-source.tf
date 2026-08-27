data "starrocks_plugin" "audit" {
  name = "AuditLoader"
}

output "plugin_type" {
  value = data.starrocks_plugin.audit.type
}

output "plugin_status" {
  value = data.starrocks_plugin.audit.status
}

output "plugin_version" {
  value = data.starrocks_plugin.audit.version
}
