# Beads Task Graph Plan

This is a suggested task graph for the implementing agent. It is provided as markdown so the next agent can adapt it if the current bead graph has changed.

```bash
#!/bin/bash
set -e

if [ "$(pwd)" != "/Users/punk1290/git/eino-tools" ] || [ ! -d ".beads" ]; then
    echo "Run this from /Users/punk1290/git/eino-tools; refusing to initialize a new Beads database in the wrong directory." >&2
    exit 1
fi

EPIC=$(bd create "Epic: Close coding-agent tool parity verification gaps" -t epic -p 0 --labels epic,parity --silent)
bd update "$EPIC" --status in_progress

BASELINE=$(bd create "Baseline the reported parity verification gaps by running go test, go vet, and golangci-lint from the repo root" -p 0 --labels analysis --parent "$EPIC" --silent)

LINT_FIXES=$(bd create "Fix reported golangci-lint failures in applypatch/applypatch.go and fileops/read.go with narrow justifications where suppression is used" -p 0 --labels impl --parent "$EPIC" --silent)
bd dep add "$LINT_FIXES" "$BASELINE"

SEARCH_CLASSIFIER=$(bd create "Update search/search.go so ripgrep exit code 2 classifies invalid regex as invalid_pattern and inaccessible-path/runtime failures as exec_failed" -p 0 --labels impl --parent "$EPIC" --silent)
bd dep add "$SEARCH_CLASSIFIER" "$BASELINE"

SEARCH_TESTS=$(bd create "Add search regression tests for invalid regex versus inaccessible-path ripgrep failures, including deterministic partial success with matches" -p 0 --labels testing --parent "$EPIC" --silent)
bd dep add "$SEARCH_TESTS" "$SEARCH_CLASSIFIER"

SEARCH_DOCS=$(bd create "Update docs/inventory/search.md exit-code policy to document stderr-based exit-code-2 classification" -p 1 --labels docs --parent "$EPIC" --silent)
bd dep add "$SEARCH_DOCS" "$SEARCH_CLASSIFIER"

VERIFY=$(bd create "Run final go test ./..., go vet ./..., and golangci-lint run after parity gap fixes" -p 0 --labels testing --parent "$EPIC" --silent)
bd dep add "$VERIFY" "$LINT_FIXES"
bd dep add "$VERIFY" "$SEARCH_TESTS"
bd dep add "$VERIFY" "$SEARCH_DOCS"

COMMIT_PIN=$(bd create "Commit repo code/docs fixes, pull --rebase, and capture git rev-parse HEAD as the final pin for the external response artifact" -p 0 --labels release --parent "$EPIC" --silent)
bd dep add "$COMMIT_PIN" "$VERIFY"

RESPONSE=$(bd create "Create external response artifact for the original coding-agent parity request with final pin commit, verification results, ownership decisions, and deferrals" -p 0 --labels docs --parent "$EPIC" --silent)
bd dep add "$RESPONSE" "$COMMIT_PIN"

CLOSEOUT=$(bd create "Close completed beads, bd dolt push, git push, and verify git status is up to date with origin" -p 0 --labels release --parent "$EPIC" --silent)
bd dep add "$CLOSEOUT" "$RESPONSE"

echo "Created epic $EPIC"
bd children "$EPIC"
```

Expected ready work after creating this graph: only the baseline task should be unblocked. The epic is a rollup and should remain `in_progress`.
