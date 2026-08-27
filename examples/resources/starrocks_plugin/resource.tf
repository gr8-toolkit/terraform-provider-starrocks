resource "starrocks_plugin" "audit" {
  name   = "AuditLoader"
  source = "http://starrocks-thirdparty.oss-cn-zhangjiakou.aliyuncs.com/AuditLoader.zip"
}

# Install from a local path with md5 verification
resource "starrocks_plugin" "local_audit" {
  name   = "AuditLoader"
  source = "/path/to/AuditLoader.zip"

  properties = {
    "md5sum" = "73877f6029216f4314d712086a146570"
  }
}
