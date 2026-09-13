BINARY     := terraform-provider-nxs-anomaly
GOCACHE    ?= /tmp/go-build
GOMODCACHE ?= /tmp/go-mod
GO         := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go

.PHONY: build vet test testacc lint install docs docs-check wiki release-check

build:
	$(GO) build -o $(BINARY) .

vet:
	$(GO) vet ./...

test:
	$(GO) test ./... -run "^Test[^A]" -count=1

testacc:
	@test -n "$(NXS_ANOMALY_URL)" || { echo "Set NXS_ANOMALY_URL or run scripts/run_acceptance_tests.sh"; exit 1; }
	TF_ACC=1 NXS_ANOMALY_URL=$(NXS_ANOMALY_URL) NXS_ANOMALY_API_KEY=$(NXS_ANOMALY_API_KEY) \
		$(GO) test ./internal/provider/... -run "^TestAcc" -v -count=1

lint:
	golangci-lint run ./...

# Install to local terraform plugins directory for manual testing
install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/nixys/nxs-anomaly/0.1.0/$(shell go env GOOS)_$(shell go env GOARCH)
	cp $(BINARY) ~/.terraform.d/plugins/registry.terraform.io/nixys/nxs-anomaly/0.1.0/$(shell go env GOOS)_$(shell go env GOARCH)/

# Regenerate docs/ for the Terraform Registry from the schema and examples/.
# The provider's type name is "anomaly"; the Registry name is "nxs-anomaly".
docs:
	$(GO) run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0 generate \
		--provider-name anomaly --rendered-provider-name nxs-anomaly

# Compare regenerated documentation with a temporary snapshot (works before commit).
docs-check:
	python3 scripts/check_docs.py

wiki:
	python3 scripts/generate_wiki.py

release-check:
	python3 scripts/check_release.py dist
