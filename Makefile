.PHONY: build test testacc starrocks starrocks-stop install docs tfplugindocs

build:
	go build -o ./dist/terraform-provider-starrocks

test:
	go test ./...

# Run acceptance tests against a local StarRocks instance.
#
# Starts StarRocks via Docker Compose if it is not already running, waits for
# it to be healthy, runs the tests, then leaves the container running so you
# can iterate quickly. Use `make starrocks-stop` to tear it down when done.
#
# Override the image tag:
#   make testacc STARROCKS_VERSION=4.1.1
testacc: starrocks
	TF_ACC=1 \
	STARROCKS_HOST=127.0.0.1 \
	STARROCKS_PORT=9030 \
	STARROCKS_USERNAME=root \
	STARROCKS_PASSWORD="" \
	  go test -v -run TestAcc_ -timeout 20m ./internal/provider/

# Start a local StarRocks instance and wait until it is healthy.
#   make starrocks                         # uses default version (3.5.20)
#   make starrocks STARROCKS_VERSION=4.1.1
starrocks:
	STARROCKS_VERSION=$${STARROCKS_VERSION:-3.5.20} docker compose up -d
	@echo "Waiting for StarRocks to become healthy..."
	@for i in $$(seq 1 36); do \
	  status=$$(docker inspect --format='{{.State.Health.Status}}' starrocks 2>/dev/null || echo "missing"); \
	  echo "  attempt $$i/36 — $$status"; \
	  if [ "$$status" = "healthy" ]; then echo "StarRocks is ready."; exit 0; fi; \
	  sleep 5; \
	done; \
	echo "ERROR: StarRocks did not become healthy in time."; \
	docker compose logs starrocks; \
	exit 1

# Stop and remove the local StarRocks container and its volumes.
starrocks-stop:
	docker compose down -v

install: build
	mkdir -p ~/.terraform.d/plugins/gr8-toolkit/starrocks/0.1.0/darwin_arm64
	cp dist/terraform-provider-starrocks ~/.terraform.d/plugins/gr8-toolkit/starrocks/0.1.0/darwin_arm64/

tfplugindocs:
	export GOBIN=$PWD/bin
	export PATH=$GOBIN:$PATH
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

docs: tfplugindocs
	tfplugindocs generate
