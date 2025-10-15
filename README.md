# Tic-Tac-Toe Backend

Real-time WebSocket backend that matches players, persists game state, and broadcasts updates for a multiplayer Tic-Tac-Toe experience. The service is written in Go, uses Redis for state, matchmaking, and pub/sub, and is ready to run locally or inside a container.

## Features
- Real-time gameplay over a single `/ws` WebSocket endpoint
- Redis-backed matchmaking queue, game persistence, and pub/sub fan-out
- Automatic leaderboard tracking driven by completed matches
- Minimal deployment surface: single Go binary with optional Docker image

## Prerequisites
- Go 1.24 or newer
- A Redis instance accessible to the server
- (Optional) Docker if you prefer running the pre-built container image

## Configuration
Set the following environment variables before starting the server:
- `REDIS_URL`: Connection string passed to `redis.ParseURL`, e.g. `redis://localhost:6379`
- `PORT`: HTTP port for the WebSocket server (defaults to `8080`)

When no `REDIS_URL` is supplied the app logs a fallback to `localhost:6379`, but supplying an explicit `redis://...` URL is recommended.

## Run Locally
```bash
export REDIS_URL=redis://localhost:6379
export PORT=8080

go run ./...
```
The server listens on `http://localhost:8080` and upgrades WebSocket connections at `/ws`.

To build a binary:
```bash
go build -o tic-tac-toe-be
./tic-tac-toe-be
```

## Docker Workflow
```bash
docker build -t tic-tac-toe-be .

docker run --rm \
  -p 8080:8080 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  tic-tac-toe-be
```
Adjust the Redis host to match your environment (e.g. `redis://redis:6379` when running alongside a Redis container).

## WebSocket API
- **Endpoint**: `ws://<host>:<port>/ws`
- Messages are JSON objects with a `type` string and a `payload` object.

### Client → Server Messages
- `find_match`
  ```json
  {"type": "find_match", "payload": {"playerId": "player-123", "playerName": "Jane"}}
  ```
- `move`
  ```json
  {"type": "move", "payload": {"gameId": "game-uuid", "index": 4}}
  ```
- `get_leaderboard`
  ```json
  {"type": "get_leaderboard", "payload": {}}
  ```

### Server → Client Messages
- `match_found`: Broadcast when two queued players are paired. Payload mirrors the `Game` schema below.
- `game_update`: Sent after every valid move and on game completion.
- `leaderboard_update`: Returns the top leaderboard entries on request.

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
`status` transitions through `playing`, `win_x`, `win_o`, or `draw`.

## Game Flow & Persistence
- Matchmaking places players into a Redis list/sets (`matchmaking:queue`, `matchmaking:in_queue`, `players_in_game`).
- When two players are paired, a new `Game` record is stored at `game:<id>`.
- Moves are validated server-side, applied to the board, and persisted back to Redis.
- Completed games update the `leaderboard:wins` sorted set; display names are cached under `player:names`.
- Game updates are published through Redis pub/sub and routed to both participants.

## Project Layout
- `main.go`: Program entrypoint, HTTP server, and middleware wiring.
- `client.go`: WebSocket lifecycle, message routing, and move handling.
- `matchmaking.go`: Redis-backed matchmaking loop.
- `game.go`: Game domain model, rules, and persistence helpers.
- `leaderboard.go`: High score tracking helpers.
- `pubsub.go`: Subscribes to `game:*` channels and fans out updates.
- `redis.go`: Redis client initialization and connectivity checks.
- `Dockerfile`: Multi-stage build producing a minimal deployment image.

## Development Notes
- Redis must be running before the server starts or the process exits.
- `github.com/rs/cors` defaults allow all origins—tighten this for production.
- No automated tests are provided yet; `go test ./...` is the expected entrypoint when they are added.

Have fun building clients on top of the WebSocket API, and feel free to tailor the matchmaking or leaderboard logic to your needs.
