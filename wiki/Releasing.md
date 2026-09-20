# Releasing to the Terraform Registry

## Publication identity

| Item | Value |
|------|-------|
| Public GitHub repository | `nixys/terraform-provider-nxs-anomaly` |
| Registry address | `registry.terraform.io/nixys/nxs-anomaly` |
| Terraform local name / resource prefix | `anomaly` / `anomaly_` |
| Binary prefix | `terraform-provider-nxs-anomaly` |
| Protocol | `6.0` |
| Targets | Linux, macOS, Windows; amd64 and arm64 |

The repository must be public and named `terraform-provider-nxs-anomaly`.
Its release tags must be valid SemVer prefixed with `v`, for example `v0.1.0`.
The example version is illustrative; choose an unused version greater than the
last published release. A draft GitHub release is not ingested by the Registry.

## One-time account setup

1. Confirm the public GitHub repository exists and the maintainer can administer
   it and the `nixys` Registry namespace. Enable GitHub Actions and the wiki.
2. Generate or select a dedicated RSA signing key. The Registry publishing guide
   currently accepts RSA/DSA keys, not ECC. Keep the private key outside Git.
3. Export the ASCII-armored **public** key with `gpg --armor --export KEY_ID` and
   add it to the Registry namespace's signing keys.
4. Add the armored private key as GitHub Actions secret `GPG_PRIVATE_KEY` and its
   passphrase as `GPG_PASSPHRASE`. The release workflow obtains the fingerprint
   from the imported key. Never add either secret to repository files.
5. If releasing through the internal mirror job, configure its existing
   `GITHUB_KEY` deploy key with write access.

## Prepare the release commit

```bash
make build vet test
make docs
make docs-check
NXS_ANOMALY_URL=http://localhost:8080 NXS_ANOMALY_API_KEY=test-key make testacc
```

Check the complete example against a disposable service. Update version
constraints if required, regenerate documentation, and review the diff.
The release workflow repeats build, unit, and documentation checks on the tag.
Acceptance tests run separately and must be confirmed before tagging.

Check `.goreleaser.yml` with GoReleaser v2. Run an unsigned local package rehearsal:

```bash
goreleaser check
goreleaser release --snapshot --clean --skip=publish,sign
python3 scripts/check_release.py dist --allow-unsigned
```

A snapshot is a local rehearsal, not a publishable version. Run these commands in
a clean checkout for the final rehearsal. Review generated archives and checksums.
The release verifier checks the manifest, package matrix, binary names, and hashes.
It verifies the detached GPG signature unless `--allow-unsigned` is supplied.

## Create and inspect the draft

For the internal mirror process, tag the reviewed internal commit, wait for the
`build:binary` job, then run the existing manual `publish:github` job. That job
exports tracked sources, excludes internal files, and pushes a release branch
and tag. Merge the public release branch so `main` contains the release sources.
For direct public development, push the reviewed commit and its release tag to
GitHub. Do not mix both approaches for the same version.

A `v*` tag triggers `.github/workflows/release.yml`, which creates a signed
**draft** release. Inspect these assets (VERSION has no leading `v`):

- Six `terraform-provider-nxs-anomaly_VERSION_OS_ARCH.zip` archives.
- `terraform-provider-nxs-anomaly_VERSION_manifest.json` declaring protocol 6.0.
- `terraform-provider-nxs-anomaly_VERSION_SHA256SUMS`, covering every ZIP and the manifest.
- `terraform-provider-nxs-anomaly_VERSION_SHA256SUMS.sig`, a binary detached GPG signature.

Each ZIP contains `terraform-provider-nxs-anomaly_vVERSION` (plus `.exe` on
Windows). Download the draft assets into an empty directory and run:

```bash
python3 scripts/check_release.py /path/to/downloaded-assets
```

Import the public signing key into your local GPG keyring first. Match its
fingerprint with the key registered in Terraform Registry. Review release notes,
license, docs, and the final commit before publishing the draft.

## Publish and verify

1. Publish the reviewed GitHub draft release.
2. For the first release, sign in to Terraform Registry, choose **Publish > Provider**,
   and select the namespace and repository. Complete the account prompts.
3. Confirm the version, platforms, signing key, all 8 resources, all 18 data sources,
   and guides appear. Registry documentation comes from the tagged `docs/` tree;
   the GitHub wiki is a separate documentation surface.
4. In a clean directory without development overrides or a local plugin mirror,
   use the published version with `source = "nixys/nxs-anomaly"`, then run
   `terraform init` and `terraform providers schema -json`. Confirm installation
   verifies the expected signature. Run a plan against a disposable service.
5. Publish the prepared wiki pages using the procedure below.

Subsequent finalized GitHub releases are ingested through the Registry webhook.
If ingestion fails, inspect the release assets and webhook, then use the Registry
provider settings' Resync action as described in the official publishing guide.
Never replace an existing release tag or its assets. Publish a new version for fixes.

## Wiki publication

Generate portable English wiki pages from the committed reference and guides:

```bash
make wiki
```

The reviewed pages live in `wiki/`, including `Home.md` and `_Sidebar.md`.
Clone the target repository's `.wiki.git` repository, copy `wiki/*.md` into its
root, review, commit, and push. GitHub may require creating the first page in its
web interface before its wiki Git repository becomes available.
The pages use relative `.md` links and can also be read directly in the source repository.
Regenerate the wiki whenever reference documentation or release instructions change.

## Sources

- [HashiCorp: publishing providers](https://developer.hashicorp.com/terraform/registry/providers/publishing)
- [HashiCorp: provider documentation](https://developer.hashicorp.com/terraform/registry/providers/docs)
- [Registry documentation preview](https://registry.terraform.io/tools/doc-preview)
