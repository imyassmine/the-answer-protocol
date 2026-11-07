[![CI](https://github.com/imyassmine/tap/actions/workflows/ci.yml/badge.svg)](https://github.com/imyassmine/tap/actions/workflows/ci.yml)

# TAP — The Answer Protocol

A multiplayer text adventure (MUD) written in Go: one TCP server hosting a shared world, and two interchangeable clients (terminal and browser) speaking the line-based **RFC 42TAP** protocol.

Built during the 42 common core as a two-person project — standard library only, no external dependencies.

## Features

- **Shared 8-room world** loaded from `data/world.json` (rooms, exits, items, NPCs, quests) and validated at startup.
- **Multiplayer**: per-room presence, global/room/group chat, floor items, groups, live player count.
- **Combat**: `ATTACK` / `DEFEND` / `FLEE` / `STATUS`, deterministic rounds, respawn on defeat.
- **Quests**: static, data-driven definitions exposed through `QUEST` / `QUESTS`.
- **Two clients**: raw-protocol CLI, and a Go-served web client with quick actions and separate room/chat/log panes.
- **Observability**: structured JSON logs (`log/slog`) for traffic, world changes and anomalies.

## Quick start

```bash
make build           # compile server + both clients into bin/
make run-server      # terminal 1 — TCP :4242
make run-client      # terminal 2 — CLI client
make run-client-gui  # terminal 3 — web client on http://127.0.0.1:8080
```

Requires Go 1.26.8+ (as pinned in `go.mod`), `make` and a C toolchain for the race-enabled `run-*` targets. Run from the repository root: the server loads `data/world.json` relatively. `make help` lists every target.

## Protocol

Commands: `CONNECT`, `LOOK`, `MOVE`, `TAKE`, `DROP`, `INVENTORY`, `TALK`, `CHAT`, `GROUP`, `ATTACK`, `DEFEND`, `FLEE`, `STATUS`, `QUEST`, `QUESTS`, `WHO`, `QUIT`.

One UTF-8 message per line (1024-byte maximum). Replies are `OK ...` or `ERR <code> <NAME>`; asynchronous events are `EVT ...`. Chat and dialogue text is sanitized. Full specification: [`rfc/protocol-rfc.html`](rfc/protocol-rfc.html) (open it locally in a browser).

```text
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

## Architecture

- **Connection model** — accept loop, one goroutine per connection, plus a writer goroutine per client draining a buffered channel. Replies and asynchronous `EVT` messages share it: ordering is preserved and a slow client cannot block other players — its queue fills, messages are dropped with a WARN, and a write deadline closes the connection if it stops reading.
- **State** — players, floor items, NPC state and groups live behind a single `sync.Mutex`; each command is atomic and race-free. Server helpers called with the lock already held are suffixed `Locked`.
- **Dispatch** — one line = one message. The command name is case-insensitive and routed to a dedicated handler (`cmdLook`, `cmdMove`, ...); handlers are grouped by feature (movement, combat, chat, groups, ...).
- **World loading** — unknown JSON fields are rejected, then `Validate()` checks the start room, exits, cross-references, NPC dialogue and quest targets/rewards. The server refuses to start on invalid data.
- **Logging** — `log/slog` JSON on stderr: INFO for traffic and world changes, WARN for anomalies such as dropped messages, ERROR for failures. Structured attributes, not formatted strings.

## Design decisions

- **One mutex, not per-room locks** — commands are short; a single lock keeps atomicity trivial to reason about and removes a whole class of ordering bugs.
- **Combat resolved inside one command** (no tick loop or timers) — deterministic and self-contained; `DEFEND` / `FLEE` add tactical choices without a turn scheduler.
- **In-memory state** — the JSON world is the source of truth; restarts reset the game, which keeps the project dependency-free.
- **RFC deviations are deliberate and documented in this README** — `TALK` / `WHO` use the RFC reply format, `GROUP JOIN` accepts any member of the target group (invites are notifications only), `QUEST` / `QUESTS` expose static definitions by id (no player progression), and `DEFEND` / `FLEE`, `EVT ROOM COMBAT` plus extra error codes extend the base protocol.

## Layout

```
cmd/server/      TCP server, dispatcher, command handlers by feature
cmd/client/      terminal client (sends raw protocol lines)
cmd/gui/         browser client (HTTP server bridging the UI to a TCP connection)
internal/world/  world model, JSON loader and validation
data/world.json  rooms, items, NPCs, quests
rfc/             protocol specification
```

## Testing

```bash
make test   # go test -race ./...
```

The suite covers world loading and validation, text sanitization, combat rules and the web client's message parsing/escaping. CI runs formatting, `go vet`, the race-enabled tests and a full build on every push.

## License

MIT — see [LICENSE](LICENSE). The protocol specification in `rfc/` is © 42 School and is not covered by the MIT license.

## Credits

Started as a two-person project with **hrezvani** during the 42 common core and maintained by [**yassabda**](https://github.com/imyassmine).
