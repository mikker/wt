---
name: release
description: Release wt by validating the repository, tagging a version, and verifying the GitHub release and Homebrew cask update. Use when asked to release or publish a new wt version.
---

# Release wt

Releases are tag-driven. Pushing a version tag runs `.github/workflows/release.yml`, which tests the project, publishes macOS, Linux, and Windows archives to GitHub Releases, and updates `Casks/wt.rb` in `mikker/homebrew-tap`.

## Prepare

1. Review `CHANGELOG.md`. Consolidate the user-visible `Unreleased` entries under the next `0.x` version and start a fresh `Unreleased` section.
2. Commit and push the changelog and all intended release changes to `main`.
3. Confirm the checkout is on clean, synchronized `main` and `origin` points at `mikker/wt`.

## Publish

Run:

```sh
mise run release
```

The release task calculates the next `0.x` version, validates the repository, runs static analysis, tests, a build, and GoReleaser's configuration check when available, then atomically pushes `main` and an annotated tag. Do not manually upload duplicate assets or edit the tap while the workflow is running.

## Verify

1. Find the release workflow with `gh run list --workflow Release --limit 5` and watch it with `gh run watch <run-id> --exit-status`.
2. Confirm `gh release view "$VERSION"` lists Darwin, Linux, and Windows archives plus `checksums.txt`.
3. Confirm `https://github.com/mikker/homebrew-tap/blob/main/Casks/wt.rb` contains the new version.
4. If Homebrew is available, refresh the tap and verify installation:

   ```sh
   brew update
   brew reinstall --cask mikker/tap/wt
   wt --help
   ```

If publishing fails, fix the cause and re-run the failed workflow. Never move or replace a published tag; release a new version if its artifacts were already consumed.
