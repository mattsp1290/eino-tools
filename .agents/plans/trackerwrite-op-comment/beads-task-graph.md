# Suggested Beads Task Graph

This is a suggested graph for the implementation agent. The agent should adapt issue titles or priorities to current repository state, but keep the dependency shape.

## Epic

```bash
EPIC=$(bd create --title="Epic: Add trackerwrite op=comment support" --description="Add optional tracker.CommentWriter support and route tracker_write op=comment for comment-capable writers while preserving close and transition compatibility." --type=epic --priority=0 --label=epic --silent)
bd update "$EPIC" --status in_progress
```

## Tasks

```bash
ANALYZE=$(bd create --title="Analyze trackerwrite comment routing baseline" --description="Inspect tracker/writer.go, trackerwrite/trackerwrite.go, trackerwrite tests, CHANGELOG.md, and docs/inventory/trackerwrite.md to confirm current unsupported op=comment behavior and existing transition capability patterns." --type=task --priority=0 --label=analysis --parent "$EPIC" --silent)

TESTS=$(bd create --title="Add trackerwrite comment capability tests" --description="Add focused tests for comment-capable writers, close-only unsupported behavior, transition-only unsupported behavior, supported-ops labels, raw body forwarding, validation, writer error mapping, and InvokableRun comment success." --type=task --priority=0 --label=testing --parent "$EPIC" --silent)

INTERFACE=$(bd create --title="Add tracker.CommentWriter interface" --description="Add an additive CommentWriter interface in tracker/writer.go embedding CloseWriter and exposing Comment(ctx, id, body string) error without changing CloseWriter, TransitionWriter, or trackerwrite.New." --type=task --priority=0 --label=impl --parent "$EPIC" --silent)

ROUTING=$(bd create --title="Route trackerwrite op=comment for comment-capable writers" --description="Add commentWriter capability detection in trackerwrite.New, generalize supportedOps ordering, split OpComment dispatch from OpLinkPR, validate non-empty body, forward raw body to Comment, and map writer errors through failedFromWriterErr." --type=task --priority=0 --label=impl --parent "$EPIC" --silent)

DESCRIPTIONS=$(bd create --title="Refresh trackerwrite schema and tool descriptions for op=comment" --description="Update schema and Info descriptions so op=comment is described as implemented when the configured writer supports comments, while op=link_pr remains unsupported." --type=task --priority=1 --label=docs --parent "$EPIC" --silent)

DOCS=$(bd create --title="Update changelog and trackerwrite inventory for comment support" --description="Record optional tracker.CommentWriter support in CHANGELOG.md and update docs/inventory/trackerwrite.md so current behavior no longer claims all comments are unsupported." --type=task --priority=1 --label=docs --parent "$EPIC" --silent)

VERIFY=$(bd create --title="Verify trackerwrite comment support" --description="Run go test ./... and go build ./..., inspect failures, and confirm close, transition, comment, and link_pr behavior match the request." --type=task --priority=0 --label=testing --parent "$EPIC" --silent)

RELEASE=$(bd create --title="Push and prepare first eino-tools release ref" --description="Commit and push the implementation, run bd dolt push, and either cut the approved first release tag or verify the pushed commit SHA is fetchable for downstream go get." --type=task --priority=0 --label=release --parent "$EPIC" --silent)
```

## Dependencies

```bash
bd dep add "$TESTS" "$ANALYZE"
bd dep add "$INTERFACE" "$ANALYZE"
bd dep add "$ROUTING" "$INTERFACE"
bd dep add "$ROUTING" "$TESTS"
bd dep add "$DESCRIPTIONS" "$ROUTING"
bd dep add "$DOCS" "$DESCRIPTIONS"
bd dep add "$VERIFY" "$ROUTING"
bd dep add "$VERIFY" "$DESCRIPTIONS"
bd dep add "$VERIFY" "$DOCS"
bd dep add "$RELEASE" "$VERIFY"
```

## Notes

- The epic is an organizational rollup only. Do not add blocking dependencies to or from the epic.
- The implementation agent should create live beads before editing and close them during session closeout.
- If tests-first is too costly for this small change, the agent may implement `INTERFACE` and `ROUTING` before `TESTS`, but the final commit must include the coverage listed in `verification-checklist.md`.

