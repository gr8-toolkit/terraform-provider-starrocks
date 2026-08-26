# ---------------------------------------------------------------------------
# Internal catalog — import only, cannot be created or destroyed.
# Use `terraform import starrocks_catalog.default default_catalog` to bring
# the built-in catalog under Terraform management.
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "default" {
  name       = "default_catalog"
  properties = {}
}

# ---------------------------------------------------------------------------
# Hive catalog — Hive Metastore
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "hive_hms" {
  name    = "hive_hms_catalog"
  comment = "Hive catalog using Hive Metastore"

  properties = {
    "type"                = "hive"
    "hive.metastore.uris" = "thrift://metastore.example.com:9083"
  }
}

# ---------------------------------------------------------------------------
# Hive catalog — AWS Glue
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "hive_glue" {
  name    = "hive_glue_catalog"
  comment = "Hive catalog using AWS Glue"

  properties = {
    "type"                                   = "hive"
    "hive.metastore.type"                    = "glue"
    "aws.hive.metastore.glue.aws-access-key" = var.glue_access_key
    "aws.hive.metastore.glue.aws-secret-key" = var.glue_secret_key
    "aws.hive.metastore.glue.endpoint"       = "https://glue.us-east-1.amazonaws.com"
  }
}

# ---------------------------------------------------------------------------
# Iceberg catalog — Hive Metastore
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "iceberg_hms" {
  name    = "iceberg_hms_catalog"
  comment = "Iceberg catalog using Hive Metastore"

  properties = {
    "type"                                    = "iceberg"
    "iceberg.catalog.type"                    = "hive"
    "iceberg.catalog.hive.metastore.uris"     = "thrift://metastore.example.com:9083"
  }
}

# ---------------------------------------------------------------------------
# Iceberg catalog — AWS Glue
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "iceberg_glue" {
  name    = "iceberg_glue_catalog"
  comment = "Iceberg catalog using AWS Glue"

  properties = {
    "type"                                   = "iceberg"
    "iceberg.catalog.type"                   = "glue"
    "aws.hive.metastore.glue.aws-access-key" = var.glue_access_key
    "aws.hive.metastore.glue.aws-secret-key" = var.glue_secret_key
    "aws.hive.metastore.glue.endpoint"       = "https://glue.us-east-1.amazonaws.com"
  }
}

# ---------------------------------------------------------------------------
# Iceberg catalog — REST catalog
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "iceberg_rest" {
  name    = "iceberg_rest_catalog"
  comment = "Iceberg catalog using a REST catalog server"

  properties = {
    "type"                  = "iceberg"
    "iceberg.catalog.type"  = "rest"
    "iceberg.catalog.uri"   = "https://iceberg-rest.example.com"
  }
}

# ---------------------------------------------------------------------------
# Hudi catalog — Hive Metastore
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "hudi" {
  name    = "hudi_catalog"
  comment = "Hudi catalog using Hive Metastore"

  properties = {
    "type"                = "hudi"
    "hive.metastore.uris" = "thrift://metastore.example.com:9083"
  }
}

# ---------------------------------------------------------------------------
# Delta Lake catalog — Hive Metastore
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "delta" {
  name    = "delta_catalog"
  comment = "Delta Lake catalog using Hive Metastore"

  properties = {
    "type"                = "deltalake"
    "hive.metastore.uris" = "thrift://metastore.example.com:9083"
  }
}

# ---------------------------------------------------------------------------
# Paimon catalog — filesystem (HDFS)
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "paimon" {
  name    = "paimon_catalog"
  comment = "Paimon catalog on HDFS (supported from StarRocks v3.1)"

  properties = {
    "type"              = "paimon"
    "paimon.catalog.type" = "filesystem"
    "paimon.catalog.warehouse" = "hdfs://namenode:8020/warehouse/paimon"
  }
}

# ---------------------------------------------------------------------------
# Elasticsearch catalog
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "elasticsearch" {
  name    = "es_catalog"
  comment = "Elasticsearch catalog (supported from StarRocks v3.1)"

  properties = {
    "type"    = "es"
    "es.nodes" = "http://es-node1.example.com:9200,http://es-node2.example.com:9200"
  }
}

# ---------------------------------------------------------------------------
# JDBC catalog — MySQL
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "jdbc_mysql" {
  name    = "mysql_catalog"
  comment = "JDBC catalog for MySQL"

  properties = {
    "type"         = "jdbc"
    "driver_url"   = "https://repo1.maven.org/maven2/mysql/mysql-connector-java/8.0.28/mysql-connector-java-8.0.28.jar"
    "driver_class" = "com.mysql.cj.jdbc.Driver"
    "jdbc_uri"     = "jdbc:mysql://mysql.example.com:3306"
    "user"         = var.jdbc_user
    "password"     = var.jdbc_password
  }
}

# ---------------------------------------------------------------------------
# JDBC catalog — PostgreSQL
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "jdbc_postgresql" {
  name    = "postgresql_catalog"
  comment = "JDBC catalog for PostgreSQL"

  properties = {
    "type"         = "jdbc"
    "driver_url"   = "https://repo1.maven.org/maven2/org/postgresql/postgresql/42.3.3/postgresql-42.3.3.jar"
    "driver_class" = "org.postgresql.Driver"
    "jdbc_uri"     = "jdbc:postgresql://pg.example.com:5432/mydb"
    "user"         = var.jdbc_user
    "password"     = var.jdbc_password
  }
}

# ---------------------------------------------------------------------------
# JDBC catalog — Oracle
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "jdbc_oracle" {
  name    = "oracle_catalog"
  comment = "JDBC catalog for Oracle (supported from StarRocks v3.2.9 / v3.3.1)"

  properties = {
    "type"         = "jdbc"
    "driver_url"   = "https://repo1.maven.org/maven2/com/oracle/database/jdbc/ojdbc10/19.18.0.0/ojdbc10-19.18.0.0.jar"
    "driver_class" = "oracle.jdbc.driver.OracleDriver"
    "jdbc_uri"     = "jdbc:oracle:thin:@oracle.example.com:1521:ORCL"
    "user"         = var.jdbc_user
    "password"     = var.jdbc_password
  }
}

# ---------------------------------------------------------------------------
# JDBC catalog — SQL Server
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "jdbc_sqlserver" {
  name    = "sqlserver_catalog"
  comment = "JDBC catalog for SQL Server (supported from StarRocks v3.2.9 / v3.3.1)"

  properties = {
    "type"         = "jdbc"
    "driver_url"   = "https://repo1.maven.org/maven2/com/microsoft/sqlserver/mssql-jdbc/12.4.2.jre11/mssql-jdbc-12.4.2.jre11.jar"
    "driver_class" = "com.microsoft.sqlserver.jdbc.SQLServerDriver"
    "jdbc_uri"     = "jdbc:sqlserver://sqlserver.example.com:1433;databaseName=MyDatabase;"
    "user"         = var.jdbc_user
    "password"     = var.jdbc_password
  }
}

# ---------------------------------------------------------------------------
# Unified catalog — query Hive, Iceberg, Hudi, and Delta Lake as one source
# (supported from StarRocks v3.2)
# ---------------------------------------------------------------------------
resource "starrocks_catalog" "unified" {
  name    = "unified_catalog"
  comment = "Unified catalog for Hive, Iceberg, Hudi, and Delta Lake"

  properties = {
    "type"                = "unified"
    "hive.metastore.uris" = "thrift://metastore.example.com:9083"
  }
}
