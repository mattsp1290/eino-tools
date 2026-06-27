# Consumer Handoff Notes

This file is context for the `local-symphony` follow-up. It is not work to perform inside `eino-tools` unless the user explicitly asks for cross-repo changes.

## Origin

The upstream request came from `local-symphony` bead `local-symphony-hosn`, deferred to 2027-01-01 and marked P4:

```text
Option A (later): eino-tools op=transition + agent-facing transition tool
```

The design lives in:

```text
~/git/local-symphony/.agents/plans/configurable-issue-statuses/03-completion-flow.md
```

specifically the "Option A — Agent-driven transition" section.

## Consumer Shape

At the time of the request, `local-symphony` already had an internal writer capable of transitions:

```go
Transition(ctx, id, toState core.IssueState) error
```

The consumer adapter intentionally narrowed that surface to `Close` only because `eino-tools` previously accepted only `tracker.CloseWriter`.

After `eino-tools` is pinned to a commit or tag with `tracker.TransitionWriter`, `local-symphony` can add a transition-capable adapter that:

- wraps the existing `tracker.TrackerWriter`;
- exposes both `Close(ctx, id, reason string) error` and `Transition(ctx, id, toState string) error`;
- converts the string `toState` into the consumer's `core.IssueState`;
- rejects unknown target states using the consumer's existing validation/error mapping;
- updates the agent-facing prompt or runtime instructions to call `tracker_write` with `op=transition` and `toState=<configured_success_state>` after successful completion. Do not hardcode a single success state; use the configured success state for the issue type/status policy.

Then the runtime can pass that adapter to the unchanged constructor:

```go
trackerWriteTool, err := trackerwrite.New(trackeradapter.NewTransitionWriterAdapter(r.Tracker))
```

The request notes that the current local-symphony call site had drifted to:

```text
internal/runtime/runtime.go:708
```

Verify the line number again before editing that repo.

## Consumer Pin

After this repo is pushed and released or made pin-able:

```bash
go get github.com/mattsp1290/eino-tools@<tag-or-commit>
go mod tidy
```

The request recorded the older consumer pin as:

```text
github.com/mattsp1290/eino-tools v0.0.0-20260529012503-c0fcf3fb99cf
```

Do not reuse that pin; it predates the transition writer support.

## Operational Guardrail

The agent-driven transition path should run alongside the deterministic orchestrator transition backstop, not replace it. The consumer should continue to transition issues to the success state even when the model does not call `op=transition` or when a close-only writer returns `unsupported_op`.

The original request also asked for observability around fallback behavior. In `local-symphony`, add a counter or equivalent signal for `unsupported_op` / fallback events when the transition tool path does not execute.
