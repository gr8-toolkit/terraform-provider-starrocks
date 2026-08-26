# Import a database by its name.
# Note: data_quota, replica_quota, and storage_volume will be empty after
# import — add them to config manually if you want Terraform to manage them.
terraform import starrocks_database.simple analytics
