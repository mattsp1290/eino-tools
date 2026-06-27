# Verification Checklist

Use this after implementation. Beads remains the source of truth for live task state.

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

- `tracker.TransitionWriter` remains:

```go
type TransitionWriter interface {
	CloseWriter
	Transition(ctx context.Context, id, toState string) error
}
```

- `tracker.CommentWriter` is additive:

```go
type CommentWriter interface {
	CloseWriter
	Comment(ctx context.Context, id, body string) error
}
```

- `trackerwrite.New` still has this signature:

```go
func New(w tracker.CloseWriter) (*Tool, error)
```

- No `local-symphony` type or package is imported anywhere in `eino-tools`.

## Required Behavior Assertions

- `op=close` still succeeds with a close-only writer.
- `op=close` still succeeds with transition-capable and comment-capable writers.
- `op=transition` behavior is unchanged.
- `op=comment` succeeds with a comment-capable writer and returns `OutcomeSucceeded`.
- `op=comment` calls `Comment(ctx, id, body)`, not `Close`.
- `op=comment` forwards the raw body, including intentional leading/trailing whitespace.
- Empty or whitespace-only `body` fails with `ErrCategoryValidation` when the writer supports comments.
- `op=comment` returns `ErrCategoryUnsupportedOp` with a close-only writer, regardless of whether `body` is present.
- `op=comment` returns `ErrCategoryUnsupportedOp` with a transition-capable but non-comment writer.
- Comment writer errors use the same category mapping as close and transition writer errors.
- `op=link_pr` still returns `ErrCategoryUnsupportedOp`.
- Unknown ops and empty IDs still return validation failures.

## Required Supported-Ops Assertions

- Close-only unsupported message includes `supported ops: [close]`.
- Transition-capable unsupported message includes `supported ops: [close, transition]`.
- Comment-capable but non-transition-capable unsupported message includes `supported ops: [close, comment]`.
- Writer with both optional capabilities unsupported message includes `supported ops: [close, transition, comment]`.
- No supported-ops message advertises `link_pr`.

## Required Schema and Info Assertions

- `Schema()` still returns a fresh mutable copy.
- The `op` enum still includes:

```json
["comment", "transition", "close", "link_pr"]
```

- Schema properties still include `body`, `toState`, `reason`, and `prURL`.
- The `body` description no longer says `post-v1`.
- The `op` description says comments are supported when the configured writer supports comments.
- `Info.Desc` says comments are optional based on writer support.
- `link_pr` is still described as unsupported or post-v1.

## Documentation Assertions

- `CHANGELOG.md` documents the new optional `tracker.CommentWriter` support.
- Any existing changelog text saying comments are unsupported is updated or scoped to older versions.
- `docs/inventory/trackerwrite.md` no longer states that all comment calls are unsupported in current behavior.
- Documentation still states that `eino-tools` does not depend on consumer tracker types.

## Release Assertions

- The final implementation commit is pushed to the remote.
- If `v0.1.0` or another release tag is cut, the tag is pushed and verified with:

```bash
git ls-remote --tags origin refs/tags/<version>
```

- If no tag is cut, the final commit SHA is verified fetchable from the pushed branch:

```bash
commit=$(git rev-parse HEAD)
branch=$(git branch --show-current)
git ls-remote origin "refs/heads/${branch}" | grep -F "$commit"
```

- The handoff ref for `local-symphony` is explicit: either a tag or a commit SHA.

## Closeout Assertions

Before ending the implementation session:

```bash
git status --short
bd close <id> --reason="trackerwrite comment writer support implemented, verified, and pushed"
git pull --rebase
bd dolt push
git push
git status
```

The final `git status` must show the branch up to date with origin.

