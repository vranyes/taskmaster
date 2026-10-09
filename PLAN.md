# taskmaster — MCP task server

MCP server with one tool. A voice model calls it to perform a task
and gets back simple readable text.

This repo does no thinking and owns no tools. The downstream model
does the work using whatever tools it already has.

## Flow

Voice model → `perform_task({task})` → taskmaster → downstream model
→ readable `result`

## Tool

- `perform_task({task: string}) → {result: string}`
- Input is a plain task. Output is speak-ready text.
- No discovery, no capability list, no routing logic here.

## Rules

- Cap task/result size.
- Timeout per call, always return one result or error. Never hang.
- No secrets or caller content in logs.
- Model never sees credentials. The tool executor (voice-bridge)
  attaches identity server-side; the model emits `{task}` only.

## Auth (decided)

- Edge→gateway: per-call short-lived JWT minted by voice-bridge
  (sub = phone/user, aud = taskmaster, exp minutes), verified by
  taskmaster. Replaces the static `TASKMASTER_TOKEN` idea: a shared
  secret cannot bind a call to a user.
- Gateway→LibreChat: per-user Remote Agents API key looked up by
  caller phone number. Chat completions `model` = voice agent ID.
  LibreChat v0.8.8 has no impersonation or run-as; key identity
  IS the user. Token spend tracks to them.
- Phone→user map + keys live in an enrollment service store (PG),
  not Kanidm (person schema has displayname/legalname/mail only,
  no phone attr) and not SealedSecrets (runtime per-user material).
- Enrollment (separate service): Kanidm login → Telnyx SMS OTP
  (number ownership is a real attack path, not optional) → user
  pastes their LibreChat API key, verified via
  `GET /api/agents/v1/models`, stored encrypted.
- Kanidm has no refresh tokens / `offline_access` (upstream
  kanidm/kanidm#4034 open): no live user token exists over PSTN,
  so the OIDC-bearer phone path is impossible. Revisit if that
  ever ships (Kanidm currently floats `:latest`).
- Fallback if per-user is too much: single voice-agent identity.
  Breaks per-user downstream tools (e.g. IMAP OAuth mailboxes).
- Headless calls need explicit `toolApproval` allows: deny/ask
  blocks instead of prompting. Public route stays gated until
  the handler enforces auth.

## Build

1. Validation + caps. (done)
2. Downstream client + fake. (done)
3. MCP wiring for `perform_task`. (done, HTTP + 30s default)
4. Live verify: on hold pending downstream endpoint.
5. Handler auth: verify edge JWT, per-user key lookup, fail-closed.
6. Enrollment service + Kanidm client + Telnyx OTP (separate).
