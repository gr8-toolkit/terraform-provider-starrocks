terraform {
  required_providers {
    starrocks = {
      source = "gr8-toolkit/starrocks"
    }
  }
}

provider "starrocks" {
  host     = "localhost"
  port     = 9030
  username = "root"
  password = "password"
}
