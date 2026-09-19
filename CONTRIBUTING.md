# Contributing

Use Go 1.27.1 or newer (the minimum in `go.mod`), GNU Make, Python 3, and
Terraform CLI. The provider uses Terraform Plugin Framework and protocol 6.

## Local checks

```bash
make build vet test
make docs
make docs-check
```

Documentation generation uses a pinned tfplugindocs version. Resource schemas
and `examples/` are the source of generated reference pages. Guides live in
`templates/guides/` and are copied into `docs/guides/` by tfplugindocs.
Do not edit generated pages directly. Commit generated changes with their source.

All user-facing text, code comments, documentation, and examples use English.
Add an example for every new resource or data source and an `import.sh` for every
importable resource. Keep the full example in `examples/complete/` valid.
The local provider name is `anomaly`; the Registry type is `nxs-anomaly`.

## Acceptance tests

Use a disposable nxs-anomaly instance. Tests create and delete real objects.

```bash
NXS_ANOMALY_URL=http://localhost:8080 \
NXS_ANOMALY_API_KEY=test-key make testacc
```

`make testacc` requires the URL and sets `TF_ACC=1`. To start a disposable
PostgreSQL container and build the service automatically, use:

```bash
bash scripts/run_acceptance_tests.sh
```

The script requires Docker, curl, Git, and Go. Set `NXS_ANOMALY_BINARY` to use a
prebuilt service binary, or `NXS_ANOMALY_REPO` to select a service repository.
For an existing instance, set `NXS_ANOMALY_URL` and `NXS_ANOMALY_API_KEY`.

## Development override

Build with `make build` and configure `~/.terraformrc` (Windows:
`%APPDATA%\terraform.rc`):

```hcl
provider_installation {
  dev_overrides {
    "nixys/nxs-anomaly" = "/absolute/path/to/provider-checkout"
  }
  direct {}
}
```

For a configuration that only uses this provider, run `terraform validate` or
`terraform plan` directly. Initialization still queries the Registry even with a
development override. Other providers and modules may require initialization.

## Submitting changes

Describe the problem, resulting behavior, and checks performed. Add meaningful
tests for behavior changes. Run `gofmt` on changed Go files. Avoid changing
resource names or attribute types without a documented state migration.
See [RELEASING.md](RELEASING.md) for package and publication checks.
