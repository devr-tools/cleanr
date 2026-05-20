# Homebrew Core Prep

This repository is prepared to generate a Homebrew formula shaped for `homebrew/core`.

The generated formula now:

- downloads the tagged source tarball instead of platform-specific release binaries
- builds `cleanr` from source with `go build`
- injects the tagged version into `cleanr version`

Generate the formula for a tagged release with:

```bash
make homebrew-formula VERSION=vX.Y.Z REPOSITORY=owner/name SOURCE_SHA256=<sha256>
```

If the repository has an SPDX license identifier ready, include it:

```bash
make homebrew-formula VERSION=vX.Y.Z REPOSITORY=owner/name SOURCE_SHA256=<sha256> HOMEBREW_LICENSE=MIT
```

The release workflow computes `SOURCE_SHA256` from the GitHub tag archive automatically and writes `dist/releases/<version>/cleanr.rb`.

## Remaining Submission Blockers

- choose and commit an open-source license file for the repository
- pass the SPDX identifier with `HOMEBREW_LICENSE=<identifier>` when generating the formula
- confirm `cleanr` meets Homebrew's `homebrew/core` notability and policy requirements
- open a formula PR against `Homebrew/homebrew-core`

## Expected Submission Shape

When you are ready to submit:

1. Tag a release in this repository.
2. Run the release workflow or `make homebrew-formula` locally with the source archive SHA.
3. Use the generated `cleanr.rb` as the starting point for a `homebrew/core` PR.
4. Expect Homebrew maintainers to review the formula and rebuild bottles in their own CI.
