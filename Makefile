.PHONY: build test install docs

build:
	go build -o ./dist/terraform-provider-starrocks

test:
	go test ./...

install: build
	mkdir -p ~/.terraform.d/plugins/gr8-toolkit/starrocks/0.1.0/darwin_arm64
	cp dist/terraform-provider-starrocks ~/.terraform.d/plugins/gr8-toolkit/starrocks/0.1.0/darwin_arm64/

tfplugindocs:
	export GOBIN=$PWD/bin
	export PATH=$GOBIN:$PATH
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

docs: tfplugindocs
	tfplugindocs generate
