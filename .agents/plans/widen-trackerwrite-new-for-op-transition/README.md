# Widen `trackerwrite.New` for `op=transition`

Source request: `/Users/punk1290/.agents/projects/eino-tools/requests/2026-06-20-widen-trackerwrite-new-for-op-transition.md`

Project: `eino-tools`

Change type: additive feature / release readiness

## Goal

Allow `tracker_write` to execute `op=transition` when the configured tracker writer supports transitions, while keeping `trackerwrite.New(tracker.CloseWriter)` and all close-only behavior backward compatible.

The original ask came from the `local-symphony` configurable issue statuses work. In that consumer, the orchestrator already has a deterministic transition backstop, so this is an optional ergonomics layer that lets the agent express the transition directly.

## Current Status

As of this planning pass, the current `eino-tools` tree already appears to contain the code-level implementation:

- `tracker/writer.go` defines `TransitionWriter`, embedding `CloseWriter` and adding `Transition(ctx, id, toState string) error`.
- `trackerwrite.New(w tracker.CloseWriter)` keeps its original public signature and detects `tracker.TransitionWriter` with a type assertion.
- `trackerwrite.Run` routes `OpTransition` to `Transition` when the writer supports it.
- Close-only writers still receive `unsupported_op` for `op=transition`.
- `op=comment` and `op=link_pr` still return `unsupported_op`.
- `trackerwrite/trackerwrite_test.go` contains unit coverage for transition success, close-only fallback, validation, writer errors, schema shape, and invokable execution.

The implementation appears to have landed via recent history that includes:

```text
b93c007 Merge branch 'feature/trackerwrite-transition-writer' of github.com:mattsp1290/eino-tools
```

The next agent should therefore verify the current implementation against the request before making code edits. If verification passes, the remaining work is release readiness: ensure the final commit is pin-able, cut or document a tag/commit, and hand off the consumer update path.

## Affected Areas

- `tracker/writer.go`: exported writer interfaces.
- `trackerwrite/trackerwrite.go`: constructor capability detection, transition dispatch, schema and tool descriptions, unsupported-op messages.
- `trackerwrite/trackerwrite_test.go`: close-only and transition-capable writer behavior.
- `CHANGELOG.md` or release notes: document the additive `TransitionWriter` capability if the repo normally records public API changes there.
- Git tags / pushed commit: make the result consumable by `local-symphony`.

## Design Decisions

- Keep `tracker.CloseWriter` unchanged.
- Keep `trackerwrite.New(w tracker.CloseWriter)` unchanged. Existing close-only callers must compile without edits.
- Add transition support through optional capability detection. A value passed to `New` that also implements `tracker.TransitionWriter` enables `op=transition`.
- Keep the `Transition` boundary type as `string`; do not import or depend on `local-symphony` issue-state types.
- Check transition writer capability before validating `toState`. A close-only writer should get terminal `unsupported_op` for `op=transition`, not a retryable shape/validation hint.
- Trim `toState` before passing it to the writer. Reject empty or whitespace-only values with the existing validation result category.
- Route transition writer failures through the same writer-error mapping used by `op=close`.
- Leave `op=comment` and `op=link_pr` unsupported.

## Constraints

- No signature-breaking change to `trackerwrite.New`.
- No dependency from `eino-tools` to `local-symphony`.
- No replacement of the consumer's orchestrator transition backstop.
- Preserve the structured result envelope and existing error categories.
- Use Beads for live task tracking during implementation.
- End state must be committed and pushed according to the repo session protocol.

## Success Criteria

- `tracker.CloseWriter` and `trackerwrite.New(tracker.CloseWriter)` remain backward compatible.
- `tracker.TransitionWriter` is available as an additive interface.
- With a transition-capable writer, `tracker_write {"op":"transition","id":"T-1","toState":"accepted"}` calls the writer and returns `OutcomeSucceeded`.
- With a close-only writer, the same op returns `unsupported_op`.
- Empty or whitespace-only `toState` returns validation failure when the writer supports transitions.
- `op=comment` and `op=link_pr` continue to return `unsupported_op`.
- `go test ./...` and `go build ./...` pass.
- A tag or pushed commit exists that `local-symphony` can pin with `go get github.com/mattsp1290/eino-tools@<tag-or-commit>`.
