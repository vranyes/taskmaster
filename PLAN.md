# taskmaster — ask_home task client

Voice calls are OpenAI Realtime (expensive). Thinking is home models
(cheap). This repo is the bridge side of that split: it lets the voice
model delegate arbitrary tasks to a dedicated home LibreChat **voice
agent** and get back speak-ready text.

## Flow

Caller → Telnyx → voice-bridge → OpenAI Realtime → `perform_task({task})`
→ taskmaster client → LibreChat voice agent → speak-ready result →
OpenAI speaks it briefly.

Cost rule: the voice model classifies and speaks short replies only.
Retrieval, reasoning, and summarization live home-side.

## Tool surface (deliberately one tool)

- `perform_task({task})` — static definition, no discovery, no capability
  enum. Routing (which agent/tools run) is LibreChat's job.
- No `list_capabilities`: capability data barely changes within a call,
  and every discovery call burns realtime tokens for nothing.

## Security (fail-closed)

- Least privilege lives home-side: a dedicated **voice agent** with only
  phone-safe tools. The service credential may invoke only that agent.
- Bridge allowlists nothing by name anymore (no capability bit), but
  still: task/result size caps, per-call timeout, no secrets or caller
  content in logs beyond redacted shape, exactly-one-output per tool
  call (success or model-visible error — never a hung turn).
- Dispatch is bounded and async off the audio pace loop: a slow home
  model must never stall playout or barge-in.

## Client slices (TDD, atomic commits, `make test-race` before commit)

1. Domain: task/result validation + byte caps.
2. Port + fake: `AgentCaller` interface (run task, await result, ctx
   timeout) + scripted fake.
3. LibreChat client: service auth, run/await against the voice agent,
   timeout enforcement, result extraction. httptest-backed.
4. ToolLoop wiring: `perform_task` alongside `end_call`; unknown tools
   denied with model-visible errors.
5. Metrics (`taskmaster_*`, bounded labels) + env (`TASKMASTER_*`) +
   SealedSecret gitops.
6. Live verify: "do I have any emails?" → home summary → short spoken
   reply; wrong-path tests (timeout, deny, oversize result).

## Open questions (Slice 0.5, must answer before slice 3)

- LibreChat version + service-callable run/await path for the voice
  agent (or session-cookie-only? then fallback: point at the underlying
  model provider with an agent-equivalent prompt).
- Service credential type + which tools the voice agent gets.
- P50/P99 home-model latency (validates the per-call timeout budget).
