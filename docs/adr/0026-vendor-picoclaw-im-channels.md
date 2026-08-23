# Vendor picoclaw's Feishu and WeCom channels rather than depending on or rewriting them

Status: Accepted

Relates to ADR-0021 (licensing posture) and ADR-0022 (first-party Go agent core).

## Context

`docs/architecture/ARCHITECTURE_BLUEPRINT.md` describes an `internal/gateway`
package carrying Feishu and WeCom adapters. No such code exists — a repository
sweep for `feishu` / `wecom` / `lark` returns hits only inside `.gomodcache`.
The blueprint's directory map lists the package in the same notation as packages
that are actually built, so a reader cannot tell plan from fact.

The blueprint also names [`sipeed/picoclaw`](https://github.com/sipeed/picoclaw)
as the agent-architecture reference baseline. Examining that repository changes
the build-versus-borrow calculation for the channel layer specifically:

- **Language and version match exactly**: Go 1.25, the same as `core-service`.
- **Licence is MIT** (`Copyright (c) 2026 PicoClaw contributors`) — permissive,
  no copyleft, compatible with the BSL / Fair Source posture ADR-0021 adopted
  and with the 私有化部署 delivery model that posture exists to protect.
- **`pkg/channels/` contains 21 channel implementations**, including
  `feishu/` (with `feishu_32.go` / `feishu_64.go` build tags) and `wecom/`,
  behind a common `Channel` interface with a shared `BaseChannel`.
- Both target channels use **outbound WebSocket** long connections rather than
  inbound HTTP webhooks. WeCom is described upstream as a "WebSocket-based AI
  Bot channel with route persistence"; Feishu runs in WebSocket/SDK mode.

That last point matters more than it first appears. The original plan for this
package assumed inbound webhooks, and therefore assumed a publicly reachable
callback URL, inbound signature verification, and a meaningful attack surface
that justified deferring the work entirely. Outbound WebSocket has none of
those properties: the service dials out, and there is no public ingress to
defend.

What picoclaw cannot supply is the part that carries all the risk. It is a
*personal* assistant — one user, one machine. Its identity model is
`IsAllowed(senderID)`, a do-not-disturb allow-list. This product has multiple
legal entities, regions and roles, and 底线 1 (cross-entity isolation) means a
single mis-scoped message is a data leak across tenants.

Three options were considered.

**Depend on the upstream module** (`go get sipeed/picoclaw`) is cheapest to
start. But `pkg/channels` is coupled to `pkg/bus`, and taking the dependency
pulls `bus`, `session`, `identity` and `routing` into a repository that already
has its own agent loop, session store and tenancy model from ADR-0022. Two
session abstractions and two identity abstractions in one binary is a durable
source of confusion, and upstream is free to reshape any of them.

**Clean-room rewrite** avoids all coupling and all licence bookkeeping, but
re-derives protocol framing, reconnect and media handling that upstream already
has working across two channels — work that carries no domain value and that we
cannot validate without real tenant credentials.

**Vendoring a narrow slice** takes the protocol code and leaves the
abstractions.

## Decision

### 1. Vendor `pkg/channels/{feishu,wecom}` under `internal/gateway/vendor/picoclaw/`

Scope is bounded explicitly:

| In | Out |
|---|---|
| `pkg/channels/feishu/`, `pkg/channels/wecom/` | `pkg/agent`, `pkg/bus`, `pkg/session`, `pkg/memory`, `pkg/routing` |
| The minimum of `channels`' shared skeleton those two require (`base.go`, `interfaces.go`, `errors.go`, `media.go`, trimmed to actual dependency) | `pkg/identity`, `pkg/auth`, `pkg/credential` |
| | The other 19 channels |

`bus.OutboundMessage` and `bus.SenderInfo` are rewritten as first-party types
rather than dragging in `pkg/bus` for two structs.

Each vendored file retains its upstream path, commit SHA and MIT notice in a
header comment. A `THIRD_PARTY_NOTICES` file at the repository root records the
licence. **No business logic is written inside the vendor directory** — changes
to upstream behaviour go in a wrapper, so that a future diff against upstream
stays meaningful.

### 2. The agent loop is not vendored

ADR-0022 stands unchanged: the agent core is first-party Go in
`internal/agentcore`, "not tau, not pi, not any third-party runtime". This
decision borrows a transport, not a brain. The channel layer connects to the
existing agent; picoclaw's agent, session and memory packages are explicitly
excluded above.

### 3. Channel identity resolves through the same scope resolver as JWT

A single function is the only entry point from a channel into the system:

```go
func Resolve(ctx context.Context, ref ChannelRef) (agenttools.Principal, error)
```

It returns a complete `agenttools.Principal` or an error — never a `Scope`
fragment, never a bare `legal_entity_id`, never an intermediate "user exists"
result. Callers therefore have no materials with which to assemble permissions
themselves. Internally it looks up the binding, resolves the internal user, and
**delegates to the same scope resolver the JWT path uses** — the same function,
not a copy of its logic.

Unbound channel identities are rejected. There is no default, anonymous or
fallback tenant, and no parameter through which one could be supplied.

### 4. `IsAllowed` is a do-not-disturb filter, never an authorisation decision

The vendored `IsAllowed(senderID)` / `IsAllowedSender(sender)` may run as a
coarse first pass. It returning true establishes nothing about data visibility.
Authorisation happens after `Resolve`, through `Principal.Scope`, on the same
path as every other caller.

### 5. Both rules above are enforced by architecture tests, not review

- `internal/gateway/**` (vendor included) must not construct `access.Scope` and
  must not contain a `legal_entity_id` literal.
- `internal/gateway/vendor/picoclaw/**` must not import `internal/repository`,
  `internal/services`, `internal/agenttools` or `internal/access`. Dependency
  direction is one-way: the wrapper depends on the vendor, never the reverse.
- A reverse test proves each guard fails when violated.

This follows the same mechanism as the `finmodel` and `agentcore` import
guards. The reasoning is recorded in AGENTS.md: with one human and a fleet of
agents, controls that depend on someone noticing do not hold.

### 6. Channels are wired into `main.go` but default to off

A configuration switch controls each channel; the default is off, and a channel
whose credentials are absent does not start — it logs a named reason and does
not panic or retry-loop. This mirrors the existing `worker` Compose profile,
where `agent-runner` ships wired but disabled.

Because the transport is outbound WebSocket (§ Context), shipping wired-but-off
opens no ingress. A test asserts that the default configuration starts zero
channels.

## Consequences

**Protocol work disappears; permission work does not.** The saving is real but
narrow: framing, reconnect and media handling come for free, while the binding
table, `Resolve`, and the guards in §5 are ours to write and are where every
review cycle should be spent. Reviewing this package by asking "does Feishu
connect?" would miss the entire risk.

**A third-party licence obligation enters the repository** for the first time
since ADR-0024 removed the last one. MIT's obligation is attribution only, which
§1 discharges, but the `THIRD_PARTY_NOTICES` file now needs maintaining and any
future vendoring should extend it rather than start a second mechanism.

**Upstream drift becomes a maintenance item.** Pinning by commit SHA and
forbidding in-place edits keeps a future diff tractable, but nothing pulls
upstream fixes automatically. This is accepted: the vendored surface is two
channels of protocol code, not a fast-moving dependency.

**Feishu and WeCom cards cannot render SVG.** Charts stay on the web client;
IM cards carry key figures and a parameterised deep link back into the
workstation. Rasterising to PNG would add an image dependency for a 3-inch
screen, and was rejected on 2026-08-23.

**The blueprint overstates what exists.** `docs/architecture/ARCHITECTURE_BLUEPRINT.md`
should mark `internal/gateway/` and its siblings as planned rather than built,
or readers will keep mistaking the map for the territory.

## Non-goals

- Not a decision to adopt picoclaw's runtime, agent loop, bus or session model.
  ADR-0022 governs those and is unchanged.
- Not a decision about the other 19 upstream channels. Slack, Telegram, DingTalk
  and the rest can be added later through the same seam; none is in scope now.
- Not a decision about credential storage or rotation. Configuration keys are
  defined; the values and their handling are an operations concern.
- Not a commitment to keep vendoring. If the wrapper ever grows larger than the
  vendored code, a clean-room rewrite becomes the cheaper option and this ADR
  should be revisited.
