# Changelog

## 0.0.16

- Fix release validation: exclude the checksum signature from `SHA256SUMS`, which
  GoReleaser added when refreshing checksums after signing and which invalidated
  the signature.
- Add the reusable on-call module sample in `examples/module`.
- Add the project logo to the README.

## 0.0.17

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
- Remove the GitHub Actions acceptance-test job: it has no credentials for the
  private nxs-anomaly service repository, which only GitLab CI can access via
  `CI_JOB_TOKEN`.

## 0.0.18

- Move the README title above the project logo.

## 0.0.19

- Add a Terraform Registry badge to the README.

## 0.1.1

- Fix `terraform init` from the README and examples: they required `~> 0.1`,
  which no published release matches; they now require `~> 0.0`.
- Fix "Provider produced inconsistent result after apply" for `anomaly_schedule`
  shifts and rotation and for `anomaly_schedule_override` when a time is written
  with an offset other than the schedule's timezone (for example `Z` in a
  `Europe/Moscow` schedule). The configured spelling is kept whenever the API
  returns the same instant; a different instant is still reported as drift.
- Document that, after `terraform import`, time attributes written with another
  offset than the API returns show a one-time in-place update. Terraform requires
  a required attribute's plan to match the configuration, so the provider cannot
  suppress it.

## 1.1.0

- Start the 1.x release line. The README, the usage guide and every example now
  require `~> 1.1`: the previous `~> 0.0` constraint admits only 0.x releases, so
  `terraform init` would have kept installing 0.1.1.
- No provider behaviour changes since 0.1.1.

## 1.1.1

- `anomaly_user.on_duty` is no longer forced to `false`. Unset in configuration it
  is not sent and follows the API, so a responder who takes duty through the API
  or UI is not taken off duty by the next `apply`; set, Terraform manages it.
- The complete example, the usage guide and the module example define the "Ops
  Weekly Rotation" as a `rotation`. Two weekly-recurring shifts a week apart both
  recur every week, so from the second week both people were on call at once.
- With nxs-anomaly releases that stop returning `webhook_secret`, the configured
  secret stays in state; out-of-band changes to it can no longer be detected.
- Deleting a user, team, schedule or escalation chain that paging still uses is
  refused by those releases with 409. Terraform orders deletes by dependency, but
  replacing such an object in place needs `create_before_destroy`.

## 1.1.2

- A data source looked up by `name` now fails when more than one object carries
  that name, instead of silently returning whichever came first.
- `anomaly_integration.heartbeat` no longer fails apply with "Provider produced
  inconsistent result" when written the way the API does not store it:
  `grace_seconds = 0` (the API stores a third of the interval) or an
  `interval_seconds` below 60 (raised to 60). The failed apply tainted the
  integration, and every later apply replaced it with a new routing key. A value
  the API only normalised is kept as written; any other difference is still
  drift.
- The module example no longer defaults `grace_seconds` to 0, which triggered the
  failure above for every integration with a heartbeat.
- The examples' `anomaly_url` and `anomaly_api_key` variables default to `null`.
  Their `"http://localhost:8080"` and `""` defaults were set values, which the
  provider prefers over `NXS_ANOMALY_URL` / `NXS_ANOMALY_API_KEY`, so the
  environment variables the comments pointed to were never read.

## 1.2.0

- Build with Go 1.27 (latest stable, 1.27.1): `go.mod` declares `go 1.27.1`, the
  GitLab pipeline runs `golang:1.27`, and the GitHub release workflow picks the
  version up from `go.mod`. Building from source needs Go 1.27.1 or newer.
- Lint with golangci-lint v2.13.2. v1.64.8 cannot read the export data of the
  Go 1.27 standard library and stopped with an internal error; v2 also reports
  unchecked `Close` errors, which are now handled explicitly.
- No provider behaviour changes since 1.1.2.

## 1.2.1

- Publish to GitHub over SSH on port 443 (`ssh.github.com`). The runner cannot
  reach `github.com:22`, so `publish:github` timed out and 1.2.0 never reached the
  public repository or the Registry. GitHub's published host keys are pinned
  instead of scanned: the scan used port 22 as well and silently left
  `known_hosts` empty.

## 1.2.2

- The README no longer says Registry installation waits for the first signed
  release or refers to a `0.1.x` series: the provider is published in the
  Registry and the examples use the 1.x line.
- The module example requires provider `>= 1.1.2, < 2.0.0` instead of
  `>= 0.0.11`. Earlier releases fail apply on a heartbeat the API normalises,
  which the module's defaults can produce.
- The README and usage guide list Go 1.27.1, the version `go.mod` requires.
- No provider behaviour changes since 1.2.1.
