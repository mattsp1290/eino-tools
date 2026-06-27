# Implementation Plan

Use this file as the execution guide for the coding agent. Track actual work with `bd`; do not use this markdown file as a live task tracker.

## 1. Start and Confirm Baseline

Run from `/Users/punk1290/git/eino-tools`:

```bash
bd ready
bd create --title="Add trackerwrite comment writer support" --description="Implement additive tracker.CommentWriter support and route tracker_write op=comment for comment-capable tracker writers." --type=feature --priority=0
bd update <id> --claim
git status --short
git log --oneline -10
git tag --list
```

Inspect the relevant surfaces:

```bash
sed -n '1,140p' tracker/writer.go
sed -n '1,380p' trackerwrite/trackerwrite.go
sed -n '1,520p' trackerwrite/trackerwrite_test.go
sed -n '1,220p' CHANGELOG.md
sed -n '1,220p' docs/inventory/trackerwrite.md
rg -n "CommentWriter|TransitionWriter|OpComment|OpLinkPR|supportedOps|body|func New\\(" tracker trackerwrite docs CHANGELOG.md
```

Do not change unrelated tool behavior while making this update.

## 2. Add the Interface

In `tracker/writer.go`, add `CommentWriter` after `TransitionWriter` or near the optional interfaces:

```go
// CommentWriter is an optional tracker mutation surface for writers that can
// close issues and post comments.
type CommentWriter interface {
	CloseWriter
	Comment(ctx context.Context, id, body string) error
}
```

Keep these surfaces unchanged:

```go
type CloseWriter interface {
	Close(ctx context.Context, id, reason string) error
}

type TransitionWriter interface {
	CloseWriter
	Transition(ctx context.Context, id, toState string) error
}

func New(w tracker.CloseWriter) (*Tool, error)
```

The boundary type for `body` is `string`; do not import consumer packages.

## 3. Detect Comment Capability

In `trackerwrite/trackerwrite.go`, add a field to `Tool`:

```go
commentWriter tracker.CommentWriter
```

Populate it in `New` alongside the existing transition assertion:

```go
t.transitionWriter, _ = w.(tracker.TransitionWriter)
t.commentWriter, _ = w.(tracker.CommentWriter)
```

Update the nearby comments so they explain both optional capabilities without implying the two are coupled.

## 4. Generalize Supported Ops

Replace the current two-branch `supportedOps()` implementation with list assembly.

Use this stable order:

```text
close, transition, comment
```

Expected outputs:

- close-only writer: `close`
- transition-only optional writer: `close, transition`
- comment-only optional writer: `close, comment`
- writer with both capabilities: `close, transition, comment`

Do not include `link_pr`.

This string is part of user-facing error messages. Existing tests assert the close-only message `supported ops: [close]`, so preserve that exact close-only value.

## 5. Route `OpComment`

Split `OpComment` out of the existing shared unsupported case.

The comment case should parallel transition:

```go
case OpComment:
	if t.commentWriter == nil {
		return failedResult(args, ErrCategoryUnsupportedOp,
			fmt.Sprintf("op %q is not supported by the configured writer; supported ops: [%s]",
				args.Op, t.supportedOps()))
	}
	if strings.TrimSpace(args.Body) == "" {
		return failedResult(args, ErrCategoryValidation, "body is required and must be non-empty for op comment")
	}
	if err := t.commentWriter.Comment(ctx, args.ID, args.Body); err != nil {
		return failedFromWriterErr(args, err)
	}
	return succeededResult(args)
```

Important details:

- Check capability before body validation.
- Use `strings.TrimSpace` only for the emptiness check.
- Pass raw `args.Body` to the writer.
- Writer errors must use `failedFromWriterErr`.
- Keep `OpLinkPR` on the "defined but not implemented" unsupported path.

## 6. Refresh Model-Facing Descriptions

Update the schema JSON and `Info.Desc` so they no longer say comment is post-v1 or unimplemented.

The `op` property description should say:

- `close` is implemented.
- `transition` is implemented when the configured writer supports transitions.
- `comment` is implemented when the configured writer supports comments.
- unsupported ops return `tool_failed{error.category=unsupported_op}`.

The `body` property description should say:

```text
Comment body. Required for op=comment.
```

The tool description should mention optional transition and comment support, and still make clear unsupported ops return `unsupported_op`.

Leave `prURL` / `op=link_pr` as post-v1 or unsupported.

## 7. Tests to Add or Update

Add a fake comment writer and, if useful, a fake writer that implements both comment and transition.

Required behavior coverage in `trackerwrite/trackerwrite_test.go`:

- `TestRunCommentSucceedsWhenWriterSupportsIt`: success result, one `Comment` call, correct id, raw body forwarded.
- `TestRunCommentPreservesRawBody`: body such as `"  verdict: pass\n"` passes raw, not trimmed.
- `TestRunCommentRequiresBodyWhenWriterSupportsIt`: empty and whitespace-only body return `ErrCategoryValidation`.
- `TestCommentOnCloseOnlyWriterIsUnsupportedRegardlessOfBody`: close-only writer gets `ErrCategoryUnsupportedOp` whether body is absent or present, and writer is not called.
- `TestCommentOnTransitionWriterIsUnsupportedRegardlessOfBody`: transition-capable but non-comment writer gets unsupported and no transition/close call.
- `TestCommentWriterWithoutTransitionAdvertisesCloseComment`: unsupported transition or link_pr message includes `supported ops: [close, comment]`.
- `TestWriterWithCommentAndTransitionAdvertisesAllSupportedOps`: unsupported link_pr message includes `supported ops: [close, transition, comment]`.
- `TestRunCommentWriterErrors`: deadline, canceled, and generic writer errors map to timeout, canceled, and unknown.
- `TestInvokableRunComment`: JSON payload with `op=comment` succeeds when the writer supports comments.
- Existing `op=link_pr` unsupported tests still pass.

Update existing tests that currently put `OpComment` in generic unsupported loops so they reflect the writer capability being exercised. For close-only writers, `OpComment` should remain unsupported. For comment-capable writers, it should be routed.

## 8. Documentation and Changelog

Update `CHANGELOG.md`:

- Add an `Unreleased` entry for optional `tracker.CommentWriter` support.
- Update the pending `v0.1.0` trackerwrite bullet so it no longer says comments are unsupported.
- Update migration notes that currently say consumers needing comments must keep those operations in their own tracker layer.

Update `docs/inventory/trackerwrite.md` if the inventory is still maintained:

- Add `CommentWriter` to the extracted interface boundary.
- Update unsupported-op behavior to say `link_pr` remains unsupported, while `comment` is optional based on writer capability.
- Update execution path notes with comment validation and raw-body forwarding.

Keep documentation edits concise; do not rewrite unrelated inventory history.

## 9. Verify

Run:

```bash
go test ./...
go build ./...
```

If the repo has a standard lint command in current HEAD, run it too. If no standard lint command exists, do not invent lint tooling as part of this change.

If a failure appears unrelated, record it in the bead before deciding whether it blocks release. Do not fix unrelated drift unless it prevents trustworthy verification.

## 10. Release or Pin

The request strongly prefers the first release tag after the implementation lands. The plan should not invent release authority during implementation.

After tests pass, commit and push the implementation:

```bash
git status --short
git add tracker/writer.go trackerwrite/trackerwrite.go trackerwrite/trackerwrite_test.go CHANGELOG.md docs/inventory/trackerwrite.md
git commit -m "Add trackerwrite comment writer support"
git pull --rebase
bd dolt push
git push
```

If the release owner approves `v0.1.0`, create and push the tag from the pushed implementation commit:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
git ls-remote --tags origin refs/tags/v0.1.0
```

If no tag is approved, hand off the pushed commit SHA:

```bash
commit=$(git rev-parse HEAD)
branch=$(git branch --show-current)
git ls-remote origin "refs/heads/${branch}" | grep -F "$commit"
```

The consumer command should be either:

```bash
go get github.com/mattsp1290/eino-tools@v0.1.0
go mod tidy
```

or:

```bash
go get github.com/mattsp1290/eino-tools@<commit-sha>
go mod tidy
```

Do not edit `local-symphony` from this repo unless the user explicitly expands the task.

## 11. Closeout

Follow the repo protocol:

```bash
git status --short
bd close <id> --reason="trackerwrite comment writer support implemented, verified, and pushed"
git pull --rebase
bd dolt push
git push
git status
```

The final `git status` must show the branch up to date with origin.

