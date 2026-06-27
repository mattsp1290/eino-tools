# Implementation Plan

Use this file as the execution guide for the coding agent. Track actual work with `bd`; do not use this markdown file as a live task tracker.

## 1. Start and Confirm Baseline

Run from `/Users/punk1290/git/eino-tools`:

```bash
bd ready
bd create --title="Ship trackerwrite transition writer support" --description="Verify and ship additive trackerwrite op=transition support for TransitionWriter-capable tracker writers." --type=feature --priority=2
bd update <id> --claim
git status --short
git log --oneline -10
```

Then inspect the relevant surfaces:

```bash
sed -n '1,120p' tracker/writer.go
sed -n '1,360p' trackerwrite/trackerwrite.go
sed -n '1,460p' trackerwrite/trackerwrite_test.go
rg -n "TransitionWriter|OpTransition|unsupported_op|toState|func New\\(" tracker trackerwrite
```

At the time this plan was written, these files already contained the requested implementation. If that remains true, do not rework it for style alone. Move directly to verification, changelog/release notes, and release/pin work.

## 2. Implement Only Missing Pieces

If the current tree does not already satisfy the request, make the smallest additive change.

In `tracker/writer.go`, add:

```go
// TransitionWriter is an optional tracker mutation surface for writers that can
// close issues and move them to a target state.
type TransitionWriter interface {
	CloseWriter
	Transition(ctx context.Context, id, toState string) error
}
```

Keep `CloseWriter` exactly as-is.

In `trackerwrite/trackerwrite.go`, keep this constructor shape:

```go
func New(w tracker.CloseWriter) (*Tool, error)
```

Add a `tracker.TransitionWriter` field to `Tool` and populate it with:

```go
t.transitionWriter, _ = w.(tracker.TransitionWriter)
```

In `Run`, route `OpTransition` as follows:

1. If `transitionWriter` is nil, return `ErrCategoryUnsupportedOp`.
2. Trim `args.ToState`.
3. If the trimmed state is empty, return `ErrCategoryValidation`.
4. Call `transitionWriter.Transition(ctx, args.ID, toState)`.
5. Convert writer errors through `failedFromWriterErr`.
6. Return the normal success result.

Keep `op=comment` and `op=link_pr` unsupported.

## 3. Preserve Schema and Messaging

The model-facing schema already declares `transition`, so keep the enum stable:

```json
"enum": ["comment", "transition", "close", "link_pr"]
```

Update descriptions only if needed so they accurately say:

- `op=close` is always implemented.
- `op=transition` is implemented only when the configured writer supports transitions.
- `op=comment` and `op=link_pr` remain unsupported.

Unsupported-op messages should not claim transition support when the configured writer is close-only. If a helper such as `supportedOps()` exists, keep it covered by behavior tests rather than testing private formatting directly.

## 4. Tests to Keep or Add

Ensure `trackerwrite/trackerwrite_test.go` covers these cases:

- Close succeeds with a close-only writer.
- Transition succeeds with a transition-capable writer.
- Transition uses `Transition`, not `Close`.
- Transition passes trimmed `toState`.
- Transition writer errors map through `failedFromWriterErr`.
- Transition with empty or whitespace-only `toState` fails validation when the writer supports transitions.
- Transition with a close-only writer returns `unsupported_op`, both with and without `toState`.
- `op=comment` and `op=link_pr` return `unsupported_op` for both writer shapes.
- `InvokableRun` accepts a transition JSON payload and returns success with a transition-capable writer.
- Schema still exposes `op`, `id`, `body`, `toState`, `reason`, and `prURL`.

Existing tests currently appear to cover these bullets. Add tests only for concrete gaps.

## 5. Documentation or Changelog

Check whether `CHANGELOG.md` is actively maintained for public API changes:

```bash
sed -n '1,220p' CHANGELOG.md
```

If the current unreleased section or project convention supports it, add a concise entry such as:

```markdown
- Added optional `tracker.TransitionWriter` support so `tracker_write` can execute `op=transition` when the configured writer implements transitions.
```

Do not add broad design prose to README unless the repo already documents individual tool API deltas there.

## 6. Verify

Run:

```bash
go test ./...
go build ./...
```

If the repo has a standard lint command in current HEAD, run it too. If no standard lint command exists, do not invent lint tooling as part of this feature unless the current branch or CI requires it.

If either command fails, determine whether the failure is caused by this change. Do not fix unrelated drift unless it blocks a trustworthy release.

## 7. Release or Pin

The consumer can use either a real tag or a pin-able commit. Prefer a tag if the repo owner wants public release semantics.

First ensure the implementation commit is pushed:

```bash
git status --short
git add <changed-files>
git commit -m "Add trackerwrite transition writer support"
git pull --rebase
bd dolt push
git push
```

If no source edits were needed because the feature already exists, use the existing pushed commit after verification. Do not create an empty commit unless the user explicitly wants one.

For a tag:

```bash
git tag -a <version> -m "Release <version>"
git push origin <version>
```

Choose `<version>` according to this repo's existing tag history:

```bash
git tag --sort=-creatordate | head -20
```

If there are no established tags or no version decision is available, record the verified commit SHA instead:

```bash
git rev-parse HEAD
```

The final handoff to `local-symphony` should say either:

```bash
go get github.com/mattsp1290/eino-tools@<version>
```

or:

```bash
go get github.com/mattsp1290/eino-tools@<commit-sha>
```

followed by:

```bash
go mod tidy
```

Do not edit `local-symphony` from this repo unless the user explicitly expands the task.

## 8. Closeout

Follow the repo protocol:

```bash
git status --short
bd close <id> --reason="trackerwrite transition writer support verified and made pin-able"
git pull --rebase
bd dolt push
git push
git status
```

The final `git status` must show the branch up to date with origin.
