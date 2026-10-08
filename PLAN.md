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

## Build

1. Validation + caps.
2. Downstream client + fake.
3. MCP wiring for `perform_task`.
4. Live verify: ask something, get readable reply back.
