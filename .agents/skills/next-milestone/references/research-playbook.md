# Repository and Current-Research Playbook

Use this reference before generating milestone candidates. Its output is an evidence set, not a roadmap.

## Repository survey

Resolve the `eino-tools` Git root and inspect only evidence that affects the frontier:

1. Read applicable `AGENTS.md`, README and contributor guidance, ADRs, inventories, and product notes.
2. Record worktree status, current revision, recent commit metadata, tags, and relevant paths. Preserve unrelated changes.
3. Survey `.agents/plans/`, `.agents/requests/`, `~/.agents/projects/eino-tools/{requests,responses}/`, and Beads. Reconcile tracker claims with committed code rather than treating status as implementation truth.
4. Trace module and package boundaries, exported types and constructors, Eino `tool.BaseTool` adapters, JSON schemas, result envelopes, catalog definitions and identities, examples, tests, CI, release notes, and supported platforms.
5. Run or identify quality commands proportionally to the candidate. Record unrelated failures separately.

Never read `.env*` values, credential stores, auth exports, private keys, shell history, raw session transcripts, or other known secret-bearing paths. Learn configuration from public types, documentation, and variable names.

Classify capabilities as:

- `implemented`: an executable public path plus a meaningful test or observed behavior proves it;
- `partial`: real code exists, but a required execution, safety, catalog, or consumer seam is missing;
- `planned`: a plan or tracker record claims the outcome without runtime evidence;
- `absent`: grounded search finds no relevant path or dependency;
- `blocked`: a named decision, request, contract, or external gate prevents safe planning;
- `unknown`: evidence is insufficient.

`implemented` and `partial` require an exact path plus a symbol, command, test, or observed behavior. A README entry, package name, dependency, schema alone, catalog metadata alone, or mock is insufficient.

## Local Eino ecosystem

Enumerate every unique resolved `~/git/eino-*` Git root, include `eino-agent`, exclude this repository, and deduplicate aliases. Shallow-inventory each repository before deciding relevance: guidance, revision and status, module identity, public packages, requests/responses, and tags. Deep-inspect public contracts and tests only for affected lanes. Treat uncommitted sibling code as provisional and never add local replacements as a planning shortcut.

Start with this ownership hypothesis, then correct it from current evidence:

| Concern | Likely owner |
| --- | --- |
| Reusable coding leaf schemas, execution, bounded results, metadata, catalog factories, and package-level safety contracts | `eino-tools` |
| Tool mounting, workspace admission, permission policy, shared per-workspace serialization, orchestration, sessions, durable execution, and model/runtime integration | `eino-agent` |
| Provider/model construction and provider-specific behavior | `eino-providers` |
| Reusable native or Wasm extensions and adapters | `eino-agent-extensions` |
| AG-UI conversion, emission, stream tapping, and client tools | `eino-agui` |
| Agent/model/tool observability and exporters | `eino-obs` |
| Terminal presentation and interaction | `eino-tui` |

Do not force a capability into the hypothesized owner when current public contracts show otherwise.

Classify each existing plan as `Product/library`, `Development-system/meta`, or `Mixed`; only product parts establish `planned` state. This skill's creation is meta work. A request records dependency need, and a response records a decision; neither proves implementation until target code and a consumable pin are verified.

## Mandatory current research

Browse every run that reaches candidate selection. Resolve current default branches and revisions before trusting seed paths. For every material source, record title, repository/publisher, direct URL, revision or update date when available, and access date.

| Lane | Required coverage | Seed authority |
| --- | --- | --- |
| Pi coding tools | Current built-in read/write/edit/bash/search or equivalent tools; tool interfaces; custom tool registration; cancellation; truncation; text and multimodal results; path and shell behavior; extension lifecycle; tests | Official Pi repository and docs at `https://github.com/earendil-works/pi` |
| OpenCode coding tools | Current tool inventory and schemas; file/search/patch/shell/task/question/web/LSP behavior where present; permission evaluation; truncation/artifacts; image/PDF and other attachment transport; concurrency; tests | Official OpenCode repository and docs at `https://github.com/anomalyco/opencode` and `https://opencode.ai/docs/` |
| Eino consumer contract | Current `eino-agent` composition, catalog mounting, execution, permissions, concurrency, persistence, events, and compatibility contracts | Local committed `~/git/eino-agent`; official remote and releases when volatile claims matter |
| Eino tool interface | Current CloudWeGo Eino tool interfaces, schema conventions, versions, releases, and migration constraints | `https://github.com/cloudwego/eino` and official documentation |
| Candidate dependencies | Any Go module, executable, protocol, API, or service named by a candidate | Current official source, documentation, releases, and security guidance |

Pi, OpenCode, local `eino-agent`, and the current Eino tool contract are mandatory. If authoritative access is unavailable, label affected claims `unverified-current`, list missing lanes, and stop before producing a plan-ready brief.

## Comparator analysis

Inventory observable behavior rather than repository names. For each relevant tool, record:

- user/agent outcome and model-facing name/schema;
- construction, registration, and runtime context;
- workspace/path, process, network, environment, and permission boundary;
- cancellation, timeout, output bounding, artifact/truncation, retry, and concurrency behavior;
- text, binary, image/PDF, typed multipart, or other model-visible result transport and its owning layer;
- errors and result representation;
- tests, platform support, and external dependencies;
- whether `eino-tools` already supplies an equivalent and whether `eino-agent` can mount it through a supported contract.

Overlapping tools are one capability lane, not separate roadmap items. Preserve meaningful differences, including minimal versus rich schemas, native library versus external executable, in-process versus subprocess work, host policy versus leaf enforcement, and direct results versus stored artifacts.

## Evidence hierarchy and clean-room boundary

Rank evidence by authority:

1. Current `eino-tools` code, tests, public contracts, and accepted decisions.
2. Committed public contracts and verified pins in local Eino-family repositories.
3. Official Eino, Go, executable, protocol, and service documentation/source.
4. Official Pi and OpenCode source/documentation for behavior and architecture lessons.

Do not copy comparator implementations, prompts, private endpoints, internal schemas, or undocumented internals. Check licenses before adapting code. Describe observable outcomes, then design independently against Go and public Eino contracts.

Label material statements `Repo fact`, `Local dependency fact`, `External fact`, `Inference`, `Proposal`, or `User decision`.
