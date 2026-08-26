# Import an external catalog by its name.
terraform import starrocks_catalog.hive hive_catalog

# Import the built-in internal catalog.
# Terraform will never attempt to destroy it — Delete is a no-op for default_catalog.
terraform import starrocks_catalog.default default_catalog
