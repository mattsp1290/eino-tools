# Beads Task Graph Plan

This is a suggested task graph for the implementing agent. It is markdown guidance; create the actual beads in the repo with `bd`.

```bash
#!/bin/bash
set -e

if [ "$(pwd)" != "/Users/punk1290/git/eino-tools" ] || [ ! -d ".beads" ]; then
    echo "Run this from /Users/punk1290/git/eino-tools; refusing to initialize or edit Beads in the wrong directory." >&2
    exit 1
fi

EPIC=$(bd create --title="Epic: Fix coding-agent parity lint portability gap" \
  --description="Make local and CI lint verification durable for the completed coding-agent parity work without relying on temporary /tmp tooling." \
  --type=epic \
  --priority=0)
bd update "$EPIC" --claim

BASELINE=$(bd create --title="Baseline reported lint portability failures" \
  --description="Run go test ./..., go vet ./..., golangci-lint --version, golangci-lint run, and the current CI go-run lint command. Record exact failures before editing." \
  --type=task \
  --priority=0)

READ_FIX=$(bd create --title="Replace formatted numbered read output with direct builder writes" \
  --description="Update fileops/read.go to avoid gosec G705 by replacing fmt.Fprintf on numbered output with strconv.Itoa plus strings.Builder writes, preserving exact output." \
  --type=bug \
  --priority=0)
bd dep add "$READ_FIX" "$BASELINE"

CI_LINT=$(bd create --title="Replace stale CI golangci-lint invocation with pinned compatible v2.x lint path" \
  --description="Update .github/workflows/ci.yml so lint no longer uses golangci-lint v2.4.0 built with Go 1.24 against this repo's Go 1.26 config. Prefer golangci/golangci-lint-action@v9 pinned to golangci-lint v2.12.2, or another verified compatible v2.x path." \
  --type=bug \
  --priority=0)
bd dep add "$CI_LINT" "$BASELINE"

DOCS=$(bd create --title="Document durable local lint expectations" \
  --description="If needed, update README.md or equivalent repo docs so future agents know the standard local lint command and required golangci-lint compatibility with Go 1.26." \
  --type=task \
  --priority=1)
bd dep add "$DOCS" "$CI_LINT"

VERIFY=$(bd create --title="Verify tests, vet, and durable lint pass" \
  --description="Run go test ./..., go vet ./..., and golangci-lint run from the repo root after code and CI changes. Confirm CI lint is pinned to a compatible v2.x path." \
  --type=task \
  --priority=0)
bd dep add "$VERIFY" "$READ_FIX"
bd dep add "$VERIFY" "$CI_LINT"
bd dep add "$VERIFY" "$DOCS"

RESPONSE=$(bd create --title="Update external coding-agent parity response artifact if pin changes" \
  --description="After final commit/rebase, update /Users/punk1290/.agents/projects/eino-tools/responses/2026-06-26-coding-agent-tool-parity-for-eino-agent.md with the new pin and durable lint verification details if HEAD changed." \
  --type=task \
  --priority=0)
bd dep add "$RESPONSE" "$VERIFY"

CLOSEOUT=$(bd create --title="Commit, push Beads, push git, and verify clean up-to-date status" \
  --description="Commit scoped repo changes, git pull --rebase, close beads, bd dolt push, git push, and verify git status reports the branch up to date with origin." \
  --type=task \
  --priority=0)
bd dep add "$CLOSEOUT" "$RESPONSE"

echo "Created epic $EPIC"
bd show "$EPIC"
```

Expected ready work after creating this graph: `BASELINE` only.
