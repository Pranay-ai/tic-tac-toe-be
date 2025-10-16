# Tic-Tac-Toe Backend

Real-time WebSocket backend for multiplayer Tic-Tac-Toe. The service is written in Go, keeps all state in Redis, and exposes a single `/ws` endpoint for gameplay events, matchmaking, and leaderboard updates.

## Highlights
- Go 1.24+ WebSocket server that multiplexes all client actions through `client.go`
- Redis-backed matchmaking queue, in-game tracking, and game persistence (`matchmaking.go`, `game.go`, `redis.go`)
- Pub/Sub fan-out so both players receive the same update stream (`pubsub.go`)
- Automatic leaderboard stored as a sorted set (`leaderboard.go`)
- Ships as a single binary or minimal Docker image (`Dockerfile`)

## System Architecture
The backend runs as one process but is split into focused components that communicate via channels and Redis.

**Runtime services (`main.go`):**
- `Hub` (`hub.go`) keeps track of WebSocket clients, handles registration/unregistration, and routes direct messages by player ID.
- `Client` (`client.go`) owns the WebSocket connection. `readPump` unmarshals messages into the shared `Message` envelope (`message.go`) and hands them to domain handlers. `writePump` streams responses back.
- Matchmaking loop (`startMatchmaking` in `matchmaking.go`) polls Redis every 3 seconds, pairs players, instantiates a new `Game`, and notifies them via the hub.
- Game services (`game.go`) enforce Tic-Tac-Toe rules, persist the board, and manage disconnect-forfeit timers.
- Leaderboard utilities (`leaderboard.go`) increment win counts and hydrate player display names.
- Redis subscriber (`pubsub.go`) listens to `game:*` channels and rebroadcasts updates through the hub so reconnects and multi-device clients stay in sync.

**Primary data flows:**
1. Client opens `/ws`; `serveWs` upgrades the connection and registers a `Client` with the `Hub`.
2. `find_match` pushes the player ID into the Redis queue and marks them as queued. Once paired, matchmaking creates a `Game`, saves it, and notifies both players with `match_found`.
3. Players take turns sending `move` messages. `handleMove` validates turn order and board state, persists the new board, and publishes a `game_update` via Redis.
4. The Redis pub/sub listener forwards the same `game_update` payload to both participants through the hub, ensuring they receive identical state whether they are currently connected or reconnecting.
5. When a game ends, the winner’s score increments in the `leaderboard:wins` sorted set and the players are removed from the `players_in_game` guard set.
6. Disconnects trigger a 30-second timer. If the player fails to reconnect (`handleReconnect`), the opponent is awarded the win.

## Data Model
- **Game (`game.go`)** stored at `game:<uuid>` as JSON with fields `playerX`, `playerO`, `board[9]`, `turn`, and `status` (`playing`, `win_x`, `win_o`, `draw`, `disconnected_x`, `disconnected_o`).
- **Redis keys (`matchmaking.go`, `leaderboard.go`):**
  - `matchmaking:queue` (list) – FIFO queue of player IDs waiting for a match
  - `matchmaking:in_queue` (set) – quick containment checks to prevent double-queueing
  - `players_in_game` (set) – prevents a player from joining while already in a game
  - `player:names` (hash) – `playerID -> display name` for leaderboard hydration
  - `leaderboard:wins` (sorted set) – win counts keyed by player ID

## WebSocket API
- **Endpoint:** `ws://<host>:<port>/ws`
- **Envelope:** every message is `{"type": "<event>", "payload": <object|array|primitive>}`

**Client → Server**
- `find_match` → `{ "playerId": "p-123", "playerName": "Jane" }`
- `move` → `{ "gameId": "game-uuid", "index": 4 }`
- `get_leaderboard` → `{}`
- `reconnect` → `{ "playerId": "p-123", "gameId": "game-uuid" }`

**Server → Client**
- `match_found` – emitted once per pairing, payload is the full `Game` struct
- `game_update` – after every valid move, reconnect, or disconnect timer resolution
- `leaderboard_update` – sorted list of `{ "name": string, "score": number }`

Example `game_update` payload:
```json
{
  "type": "game_update",
  "payload": {
    "id": "game-uuid",
    "playerX": "player-123",
    "playerO": "player-456",
    "playerXName": "Jane",
    "playerOName": "Alex",
    "board": ["X", "", "O", "", "X", "", "", "", "O"],
    "turn": "O",
    "status": "playing"
  }
}
```

## Setup
### Prerequisites
- Go 1.24 or newer
- Redis 6+ reachable from the server
- (Optional) Docker for containerized runs

### Environment variables
- `REDIS_URL` – connection string understood by `redis.ParseURL` (defaults to `redis://localhost:6379`)
- `PORT` – HTTP listen port (defaults to `8080`)

### Run locally
```bash
export REDIS_URL=redis://localhost:6379
export PORT=8080

go run ./...
```
The server listens on `http://localhost:8080` and upgrades WebSocket connections at `/ws`.

### Build a binary
```bash
go build -o tic-tac-toe-be
./tic-tac-toe-be
```

### Docker
```bash
docker build -t tic-tac-toe-be .

docker run --rm \
  -p 8080:8080 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  tic-tac-toe-be
```
Adjust the Redis host for your setup (e.g. `redis://redis:6379` inside Docker Compose).

## Development Notes
- The process calls `initRedis()` on startup and exits if Redis is unreachable.
- CORS is left permissive via `cors.Default()`. Restrict origins for production deployments.
- There are no automated tests yet; `go test ./...` is the standard entrypoint once tests are added.
- `FindMatchPayload` uses the shared `Message` envelope—ensure client payload keys match the JSON tags.

Build your client on top of the WebSocket API, or extend the matchmaking/leaderboard logic to fit your game variants.
