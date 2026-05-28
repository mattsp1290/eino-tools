# Tool Capability Request: planner

**Date:** 2026-05-28  
**Requested by:** advisor/planner (in development)  
**Priority:** needed before Phase 3 of planner implementation

---

## Background

The `planner` tool (being built in `github.com/mattsp1290/advisor`) is a CLI + MCP tool that runs a multi-turn agentic planning session. It writes plans as markdown files, then runs review and apply phases. The agent loop needs two capabilities that don't exist in eino-tools today.

---

## Request 1: `userinteract` — Ask the user a question and receive their answer

**What we need:** A tool that the agent can call to ask the engineer a question and get their response back as a string.

The critical requirement is surface-awareness: the tool must behave differently in CLI mode (where blocking on stdin is fine) vs. MCP mode (where blocking is fatal — the MCP server is single-shot per-request and cannot hold a stdin loop).

In MCP mode, the tool must signal "a user answer is pending" without blocking, so the caller can persist state and return control to the MCP host. The answer arrives on a subsequent tool invocation.

**What we do NOT need:** Multiple-choice formatting, answer validation, question templates, or any opinion about question content. The model owns question text; this tool owns only the I/O plumbing.

**Context for the implementer:** The planner's agent loop is the only consumer initially. It uses a `Surface` abstraction to distinguish CLI from MCP at runtime, so the tool needs a way to accept a surface-mode signal (not just detect stdin vs. not-a-terminal, which is insufficient when MCP also has a terminal attached).

---

## Request 2: `urlfetch` — Fetch the text content of a `file://` or `https://` URL

**What we need:** A tool that accepts a URL string and returns the raw text content of that resource. Needed to load a local HTML file (`file:///...`) for learning output style injection into the planning agent's system prompt.

Required scheme support: `file://` (local filesystem) and `https://`. Plain HTTP is optional.

**What we do NOT need:** HTML parsing, CSS stripping, JavaScript execution, cookie handling, authentication, or any HTTP features beyond reading a URL and returning its bytes as a string. The caller handles content processing.

**Why not shell `curl`?** Portability. `curl` availability and flag syntax vary across macOS, Linux CI, and container environments. A native Go implementation is more reliable in automated contexts.

---

## Dependency Assessment

If `userinteract` is complex to add (because the surface-awareness adds architectural weight), the planner will implement it inline as `internal/planner/interact/` and propose contributing it here later. `urlfetch` is simpler and a stronger candidate for immediate upstream contribution — `file://` + `https://` in pure Go is ~30 lines.

Neither request requires changes to existing eino-tools packages.
