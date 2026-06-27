# Consumer Handoff Notes

This file is context for the `local-symphony` follow-up. It is not work to perform inside `eino-tools` unless the user explicitly asks for cross-repo changes.

## Origin

The upstream request came from the `local-symphony` review-pipeline work. The originating filing bead was `local-symphony-ohz5`, and the downstream landing bead noted in the request is `local-symphony-4vgj`.

The consumer is currently pinned to a pseudo-version around commit `b93c007`, which has `TransitionWriter` and `Args.Body` but not `op=comment` routing.

## Consumer Shape

At the time of the request, `local-symphony` already had an internal tracker writer with:

```go
Comment(ctx context.Context, id, body string) error
Transition(ctx context.Context, id string, toState core.IssueState) error
Close(ctx context.Context, id, reason string) error
```

After `eino-tools` exposes `tracker.CommentWriter`, the consumer adapter should expose a `Comment(ctx, id, body string) error` method that delegates to the wrapped tracker writer.

The request identified the current runtime wiring as:

```text
internal/runtime/runtime.go:750
```

Verify the line number again before editing `local-symphony`.

## Consumer Pin

After this repo is pushed and released or made pin-able:

```bash
go get github.com/mattsp1290/eino-tools@<tag-or-commit>
go mod tidy
```

The preferred tag is the first release tag, likely `v0.1.0`, if the release owner explicitly approves it in the implementation session. If no tag is cut, use the pushed implementation commit SHA and record that no tag was approved.

## Follow-up Work in `local-symphony`

- Add `Comment(ctx, id, body string) error` to the tracker adapter that is passed into `trackerwrite.New`.
- Prefer a single adapter that satisfies both `einotracker.TransitionWriter` and `einotracker.CommentWriter` when the runtime needs both operations.
- Wire that adapter at the tracker write tool construction site.
- Update integration tests so the judge/reviewer path can post a machine-readable verdict comment before, or independently of, transition.
- Preserve consumer-side idempotency: if a comment write may have succeeded but a later transition failed, retries must not double-post the verdict comment.

## Failure Contract

`eino-tools` maps non-context comment writer errors through `failedFromWriterErr`, which currently yields `unknown` for generic errors. Consumer idempotency should treat a failed comment write as possibly already posted unless the consumer's own tracker layer can prove otherwise.

`unsupported_op` is terminal and non-retryable. If the consumer sees `unsupported_op` for `op=comment`, it likely passed a writer that does not satisfy `tracker.CommentWriter` or pinned an older `eino-tools` version.
