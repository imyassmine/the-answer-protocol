# TAP — a multiplayer text adventure in Go

[![CI](https://github.com/imyassmine/the-answer-protocol/actions/workflows/ci.yml/badge.svg)](https://github.com/imyassmine/the-answer-protocol/actions/workflows/ci.yml)
[![Go 1.26.8+](https://img.shields.io/badge/go-1.26.8%2B-00ADD8.svg)](https://go.dev/dl/)
![Standard library only](https://img.shields.io/badge/dependencies-none-success.svg)

**A shared-world MUD on a line-based TCP protocol.** One Go server hosts an 8-room world — exits,
items, NPCs, combat, quests and groups — and two interchangeable clients (terminal and browser) speak
the same RFC 42TAP lines. Standard library only, no external dependencies.

```console
$ make run-server   # TCP :4242
$ make run-client
> CONNECT alice
OK connected
EVT STATS players=1
> MOVE north
OK room=loc.tavern
> TAKE Frothy Ale
OK taken=item.ale
> CHAT GLOBAL hello
EVT GLOBAL CHAT alice hello
OK
```

## Highlights

- **Protocol-first design**: one UTF-8 message per line (≤1024 B), `OK ...` / `ERR <code> <NAME>`
  replies, asynchronous `EVT ...` events. Commands are case-insensitive and routed to a dedicated
  handler by feature; chat, dialogue and player names are sanitized.
- **Concurrency without head-of-line blocking**: one goroutine per connection plus a writer goroutine
  per client draining a buffered channel. Replies and events share the queue, so ordering is preserved;
  a client that stops reading is dropped with a WARN and closed by a write deadline instead of stalling
  the server.
- **Single source of truth**: players, floor items, NPC state and groups live behind one `sync.Mutex`,
  so every command is atomic. The world is loaded from `data/world.json` with unknown fields rejected
  and cross-references validated at startup — the server refuses to start on invalid data.
- **Gameplay**: deterministic combat (`ATTACK` / `DEFEND` / `FLEE` / `STATUS`, respawn on defeat),
  global/room/group chat, floor items, groups, and static quest definitions exposed via `QUEST` /
  `QUESTS`.
- **Observability & quality**: structured JSON logs (`log/slog`) with INFO/WARN/ERROR records,
  race-enabled `make run-*` targets, unit tests, and a GitHub Actions pipeline running gofmt,
  `go vet`, `go test -race` and a full build on every push.

## Quick start

Go 1.26.8+ and `make` (a C toolchain is needed for the race-enabled `run-*` targets). Run from the
repository root: the server loads `data/world.json` relatively.

```bash
git clone https://github.com/imyassmine/the-answer-protocol.git && cd the-answer-protocol
make build           # server + terminal client + web client into bin/
make run-server      # terminal 1 — TCP :4242
make run-client      # terminal 2 — terminal client
make run-client-gui  # terminal 3 — web client on http://127.0.0.1:8080
make test            # go test -race ./...
```

In the web client, enter a player name and press Connect — `CONNECT` is sent automatically. The full
protocol (17 commands, framing, error codes, async events) is specified in
[`rfc/protocol-rfc.html`](rfc/protocol-rfc.html) (open it locally in a browser).

## Architecture

```
cmd/server/      TCP server, dispatcher, command handlers grouped by feature
cmd/client/      terminal client (raw protocol lines)
cmd/gui/         browser client (HTTP server bridging the UI to a TCP connection)
internal/world/  world model, JSON loader and startup validation
data/world.json  8 rooms, 4 items, 4 NPCs, 2 quests
```

**Stack:** Go 1.26 · standard library · `log/slog` · `html/template` · GitHub Actions

**Design notes:** combat is resolved inside one command (no tick loop or timers) and the world is
in-memory, so restarts reset it. RFC deviations are deliberate and documented here: `TALK` / `WHO`
reply formats, `GROUP JOIN` by any member (invites are notifications), `QUEST` / `QUESTS` as static
definitions by id (no progression), plus `DEFEND` / `FLEE`, `EVT ROOM COMBAT` and extra error codes.

**Roadmap:** persistent world state, an automated RFC conformance suite, and periodic refresh in the
web client.

---
*Built at [42 Paris](https://42.fr) by [Yassmine Abdallah (@imyassmine)](https://github.com/imyassmine)
— pair project with **hrezvani**. [MIT licensed](LICENSE); the protocol spec under `rfc/` is © 42 School.*
