# Homebrew Packaging

`cleanr` now has two Homebrew-related paths:

- stable-release automation that updates `Formula/cleanr.rb` in `devr-tools/homebrew-tap`
- a local generator for producing a source-build formula suitable as a starting point for `homebrew/core`

The repository also has a pull-request Homebrew validation workflow at `.github/workflows/homebrew-validation.yml`. That workflow generates a temporary `Formula/cleanr.rb` from the current checkout, taps the repository locally, and verifies that `brew install --build-from-source` and `brew test` both pass on Ubuntu and macOS.

## Tap Sync

The release workflow now mirrors the `szr` family flow for stable tags:

- download the tagged GitHub source tarball
- compute its SHA256
- patch `Formula/cleanr.rb` in `devr-tools/homebrew-tap`
- push an automation branch when `RELEASE_PLEASE_TOKEN` is configured
- open or update the matching pull request automatically

If the token is unavailable, the push fails, or PR creation fails, the workflow writes manual follow-up instructions to the GitHub Actions summary.

That automation assumes the tap repository already contains `Formula/cleanr.rb`.

## Local Formula Generation

The local formula generator still emits a formula that:

- downloads the tagged source tarball instead of platform-specific release binaries
- builds `cleanr` from source with `go build`
- injects the tagged version into `cleanr version`

Generate the formula for a tagged release with:

```bash
make homebrew-formula VERSION=vX.Y.Z REPOSITORY=owner/name SOURCE_SHA256=<sha256>
```

If the repository has an SPDX license identifier ready, include it:

```bash
make homebrew-formula VERSION=vX.Y.Z REPOSITORY=owner/name SOURCE_SHA256=<sha256> HOMEBREW_LICENSE=Apache-2.0
```

That command writes `dist/releases/<version>/cleanr.rb`.

## Remaining Submission Blockers

- pass the SPDX identifier with `HOMEBREW_LICENSE=<identifier>` when generating the formula
- confirm `cleanr` meets Homebrew's `homebrew/core` notability and policy requirements
- open a formula PR against `Homebrew/homebrew-core`

## Expected Submission Shape

When you are ready to submit:

1. Tag a release in this repository.
2. Run the release workflow or `make homebrew-formula` locally with the source archive SHA.
3. Use the generated `cleanr.rb` as the starting point for a `homebrew/core` PR.
4. Expect Homebrew maintainers to review the formula and rebuild bottles in their own CI.
