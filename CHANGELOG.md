# Changelog

## 0.0.16

- Fix release validation: exclude the checksum signature from `SHA256SUMS`, which
  GoReleaser added when refreshing checksums after signing and which invalidated
  the signature.
- Add the reusable on-call module sample in `examples/module`.
- Add the project logo to the README.

## Unreleased

- Translate repository documentation and development instructions into English.
- Add Registry examples for all 8 resources and 18 data sources, resource import
  examples, a complete configuration, and generated wiki pages.
- Add documentation drift checks and release artifact verification; pin release tooling.
- Enable real acceptance execution with `TF_ACC=1` and correct the service database setting.
- Prevent repeated plans caused by unknown computed fields after nested-list planning.
- Keep omitted webhook secrets known after apply and preserve case-insensitive
  escalation step spelling returned in canonical form by the API.
- Declare Terraform provisioning ownership in API requests.
- Correct outdated acceptance configurations and verify maintenance-window and
  schedule-override imports, plus recreation after an external deletion.
