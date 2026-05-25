# Changelog

This project uses a hand-curated changelog.

## Unreleased

### Added

- Initial standalone module scaffolding.
- Shared `result.Outcome` enum.
- Workspace-rooted `fileops` tools.
- Ripgrep-backed `search` tool.
- `shell` execution tool.
- Close-only `tracker.CloseWriter` interface and `trackerwrite` tool.
- CI checks for tests, lint, race tests, module tidiness, and dependency
  hygiene.

### Notes

- Pre-v1.0 releases may include breaking changes in minor versions.
- Every breaking change must include a migration note in this changelog before
  release.
