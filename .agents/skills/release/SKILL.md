---
name: release
description: Release wt by validating the repository, tagging a version, and verifying the GitHub release and Homebrew cask update. Use when asked to release or publish a new wt version.
---

# Release wt

Releases are tag-driven. Pushing a version tag runs `.github/workflows/release.yml`, which tests the project, publishes macOS, Linux, and Windows archives to GitHub Releases, and updates `Casks/wt.rb` in `mikker/homebrew-tap`.

## Before tagging

1. Take the requested version without adding or removing a `v` prefix. Versions normally look like `0.1` or `0.1.1`.
2. Confirm the checkout is on `main`, has no uncommitted changes, and has an `origin` remote pointing at `mikker/wt`.
3. Fetch `origin` and confirm local `main` is neither ahead of nor behind `origin/main`.
4. Confirm the version does not already exist as a local tag, remote tag, or GitHub release.
5. Run:

   ```sh
   go test ./...
   goreleaser check # when goreleaser is installed
   ```

Do not tag a dirty, unpushed, or failing checkout.

## Publish

Create and push an annotated tag:

```sh
git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"
```

The pushed tag is the release action. Do not manually upload duplicate assets or edit the tap while the workflow is running.

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
