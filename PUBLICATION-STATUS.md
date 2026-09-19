# Publication readiness — 2026-09-12

Repository changes and wiki pages are prepared locally. No commits, tags,
remote pushes, releases, or Registry registrations were performed.

## Verified

- English documentation, comments, and examples; 8 resource pages and 18
  data-source pages, all with examples; all resources have import commands.
- 32 generated wiki pages, also copied into the local GitLab wiki checkout.
- Build, `go vet`, and unit/regression tests pass with Go 1.27.1.
- All 21 live acceptance tests pass with Terraform 1.14.0, PostgreSQL 17,
  and service commit `d124e0a7bdef5d964a5aebe7c77caa79d068b8e2`.
  The service checkout was clean; the test database was disposable.
- Tests cover all 8 resource types, maintenance-window and schedule-override
  imports, no-op plans, and recreation after a confirmed external deletion.
- `terraform fmt -check`, complete-example validation, documentation/wiki
  regeneration checks, YAML parsing, and actionlint 1.7.7 pass.
- GoReleaser 2.15.0 configuration and snapshot builds pass for all six targets.
  Archive names, binary names, protocol manifest, and SHA-256 hashes pass validation.
- Binary detached GPG signature verification passes using a disposable test key.
  The temporary key and signature were removed; this does not validate a production key.
- Public GitHub repository exists, is public, uses `main`, and has the wiki enabled.
  The Registry versions endpoint returned HTTP 404 for `nixys/nxs-anomaly`.

## Remaining publication steps

Follow [RELEASING.md](RELEASING.md) to configure the production GPG key and GitHub
Actions secrets, register the namespace signing key, select a release version,
push the reviewed sources/tag, inspect and publish the signed draft release, and
register the provider in Terraform Registry. Upload the prepared wiki pages to
the intended GitHub/GitLab wiki repositories.

Production signing secrets, account permissions, and Registry registration were
not configured or verified. The GitLab wiki Git repository was empty; the public
GitHub wiki could not be cloned anonymously. Initialize it in GitHub if needed.
A clean `terraform init` from the public Registry can only be checked after
publication; local validation used a development override.
