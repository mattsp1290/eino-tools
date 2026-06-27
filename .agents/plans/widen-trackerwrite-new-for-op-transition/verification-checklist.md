# Verification Checklist

Use this after implementation or verification. Beads remains the source of truth for live task state.

## Required Commands

Run from `/Users/punk1290/git/eino-tools`:

```bash
go test ./...
go build ./...
```

If a repo-standard lint command exists in current HEAD, run it as well.

## Required API Assertions

- `tracker.CloseWriter` remains:

```go
type CloseWriter interface {
	Close(ctx context.Context, id, reason string) error
}
```

- `tracker.TransitionWriter` is additive and accepts a string target state:

```go
type TransitionWriter interface {
	CloseWriter
	Transition(ctx context.Context, id, toState string) error
}
```

- `trackerwrite.New` still has this signature:

```go
func New(w tracker.CloseWriter) (*Tool, error)
```

- No `local-symphony` type or package is imported anywhere in `eino-tools`.

## Required Behavior Assertions

- `op=close` still succeeds with a close-only writer.
- `op=close` still succeeds with a transition-capable writer.
- `op=transition` succeeds with a transition-capable writer and returns `OutcomeSucceeded`.
- `op=transition` calls `Transition(ctx, id, toState)`, not `Close`.
- Whitespace around `toState` is trimmed before the writer call.
- Empty or whitespace-only `toState` fails with `ErrCategoryValidation` when the writer supports transitions.
- `op=transition` returns `ErrCategoryUnsupportedOp` with a close-only writer, regardless of whether `toState` is present.
- Transition writer errors use the same category mapping as close writer errors.
- `op=comment` and `op=link_pr` still return `ErrCategoryUnsupportedOp`.
- Unknown ops and empty IDs still return validation failures.

## Required Schema Assertions

- `Schema()` still returns a fresh mutable copy.
- The `op` enum still includes:

```json
["comment", "transition", "close", "link_pr"]
```

- Schema properties still include `toState`.
- Tool descriptions do not imply `comment` or `link_pr` are implemented.
- Tool descriptions do not imply `transition` is always available for close-only writers.

## Release Assertions

- The final implementation is pushed to the remote.
- Either a tag is pushed or the final commit SHA is recorded for the consumer.
- The consumer update command is known:

```bash
go get github.com/mattsp1290/eino-tools@<tag-or-commit>
go mod tidy
```

## Closeout Assertions

Before ending the implementation session:

```bash
git status --short
bd close <id> --reason="trackerwrite transition writer support verified and made pin-able"
git pull --rebase
bd dolt push
git push
git status
```

The final `git status` must show the branch up to date with origin.
