# Add `tracker_write op=comment`

Source request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-21-trackerwrite-op-comment.md`

Project: `eino-tools`

Change type: additive feature / release readiness

## Goal

Allow `tracker_write` to execute `op=comment` when the configured tracker writer supports comments, while keeping `trackerwrite.New(tracker.CloseWriter)` and all close-only and transition-capable behavior backward compatible.

This is upstream enablement for the `local-symphony` review pipeline. The consumer needs judge/reviewer agents to post a machine-readable verdict comment before, or independently of, any state transition.

## Current Status

As of this planning pass:

- `tracker/writer.go` defines `CloseWriter` and `TransitionWriter`.
- `tracker/writer.go` does not yet define `CommentWriter`.
- `trackerwrite.New(w tracker.CloseWriter)` keeps the close-only constructor signature and detects `tracker.TransitionWriter`.
- `trackerwrite.Run` routes `OpClose` and conditionally routes `OpTransition`.
- `OpComment` currently shares the unsupported path with `OpLinkPR`.
- The schema already includes `body`, but the body description still says `op=comment` is post-v1.
- `CHANGELOG.md` has an unreleased entry for `TransitionWriter` and still documents comments as unsupported in the pending v0.1.0 notes.
- `git tag --list` returned no tags when this plan was written.

## Affected Areas

- `tracker/writer.go`: exported writer interfaces.
- `trackerwrite/trackerwrite.go`: optional capability detection, supported-op labels, comment dispatch, schema descriptions, and tool description.
- `trackerwrite/trackerwrite_test.go`: capability matrix, comment routing, validation, error mapping, and invokable execution coverage.
- `docs/inventory/trackerwrite.md`: extraction inventory still describes v0.1 as close-only and comments unsupported.
- `CHANGELOG.md`: public API and release notes.
- Git release metadata: first release tag is preferred after the implementation lands.

## Design Decisions

- Keep `tracker.CloseWriter` unchanged.
- Keep `tracker.TransitionWriter` unchanged.
- Keep `trackerwrite.New(w tracker.CloseWriter)` unchanged.
- Add `tracker.CommentWriter` as an optional additive interface:

```go
type CommentWriter interface {
	CloseWriter
	Comment(ctx context.Context, id, body string) error
}
```

- Detect comment capability with a type assertion in `New`, the same way transition capability is detected.
- Treat transition and comment as independent capabilities. A writer may implement either one, both, or neither.
- Advertise supported ops in stable order: `close, transition, comment`. Omit unsupported optional ops from the label.
- Check comment capability before validating `body`, matching the transition path's terminal unsupported behavior.
- Validate emptiness with `strings.TrimSpace(args.Body)`, but pass the raw `args.Body` to `Comment`.
- Route comment writer errors through `failedFromWriterErr`.
- Leave `op=link_pr` unsupported.

## Constraints

- No signature-breaking change to `trackerwrite.New`.
- No dependency from `eino-tools` to `local-symphony` or consumer issue-state types.
- Preserve existing close and transition behavior exactly unless a test needs a supported-ops message update for a comment-capable writer.
- Preserve structured result categories and retry policy.
- Use Beads for live task tracking during implementation; this markdown is not the live tracker.
- Follow the repository closeout protocol: quality gates, commit, `bd dolt push`, `git push`, and final status.

## Success Criteria

- `tracker.CommentWriter` is exported and embeds `CloseWriter`.
- A comment-capable writer enables `op=comment`.
- `op=comment` calls `Comment(ctx, id, body)` with the raw body and returns `OutcomeSucceeded`.
- Empty or whitespace-only `body` returns `ErrCategoryValidation` when the writer supports comments.
- A non-comment writer returns `ErrCategoryUnsupportedOp` for `op=comment` and does not advertise `comment`.
- A comment-capable but non-transition-capable writer advertises `supported ops: [close, comment]`.
- A writer that implements both optional interfaces advertises `supported ops: [close, transition, comment]`.
- `op=link_pr` remains unsupported.
- Model-facing descriptions no longer describe `op=comment` as unimplemented.
- `go test ./...` and `go build ./...` pass.
- The final commit is pushed and pin-able. A first tag such as `v0.1.0` is preferred if approved by the release owner.

