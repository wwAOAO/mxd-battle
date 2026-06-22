# mxd-battle

Go service skeleton for MXD battle messages using NATS and JetStream.

## Requirements

- Go 1.22+
- Docker, if you want to run NATS locally

## Local Run

Start NATS with JetStream:

```powershell
docker compose up -d
```

Install dependencies and run the service:

```powershell
go mod tidy
go run ./cmd/server
```

## Configuration

Copy `.env.example` into your local environment or export the values directly.

| Variable | Default | Description |
| --- | --- | --- |
| `SERVICE_NAME` | `mxd-battle` | NATS client name |
| `HTTP_ADDR` | `:8080` | HTTP/WebSocket listen address |
| `NATS_URL` | `nats://127.0.0.1:4222` | NATS server URL |
| `BATTLE_STREAM` | `MXD_BATTLE` | JetStream stream name |
| `BATTLE_SUBJECT` | `battle.events.>` | Subject pattern persisted by the stream |
| `WORLD_MAPS_FILE` | `config/world_maps.json` | Room-to-map index file |

## 2D MMORPG World Sync

The service exposes one realtime wild-map scene with exactly two rooms:

```text
X
Y
```

Code layout:

```text
internal/world  world map config, map geometry, room ids, and shared collision shapes
internal/scene  scene runtime state, players, future monsters, sockets, snapshots, and sync
```

Room-to-map bindings are loaded from `config/world_maps.json` by default. Individual map files live under `config/maps`, such as `config/maps/map_x.json` and `config/maps/map_y.json`. To add or tune a map, edit its map file, then point a room at it from the index file.

HTTP endpoints:

```text
GET /health
GET /rooms
GET /rooms/X
GET /rooms/Y
```

Join a room through WebSocket:

```text
ws://127.0.0.1:8080/ws?room=X&player=role-1001&name=Hero
ws://127.0.0.1:8080/ws?room=Y&player=role-2001&name=Mage
```

Client movement messages:

```json
{"type":"input","inputX":1}
{"type":"input","inputX":0}
{"type":"jump"}
{"type":"drop"}
{"type":"portal"}
```

Test page controls:

```text
Left / Right: move
C: jump
Down + C: drop through a floating platform
Up: use portal while standing inside a portal area
```

Server events:

```text
snapshot
player_joined
player_moved
player_left
error
```

Room `X` uses map `wild_x` (`2400 x 1500`, ground `y=1500`, speed `420`) and room `Y` uses map `wild_y` (`2400 x 1400`, ground `y=1150`, speed `400`). Movement is server-authoritative: clients send input direction, then the server applies move speed, gravity, collision, and portals during the physics tick.

Players use a rectangle collider. `x` is the horizontal center of the player, `y` is the foot position, and the default body size is `52 x 96`.

Floating platforms:

| Room | Platform | Area | Solid sides | Solid ceiling |
| --- | --- | --- | --- | --- |
| `X` | `floating_stone_x_1` | `x=900,y=1350,w=360,h=44` | no | no |
| `X` | `floating_stone_x_2` | `x=900,y=1250,w=360,h=44` | yes | yes |
| `X` | `floating_stone_x_3` | `x=1600,y=1400,w=360,h=44` | no | no |
| `X` | `floating_stone_x_4` | `x=1700,y=1300,w=360,h=44` | no | no |
| `X` | `floating_stone_x_5` | `x=1820,y=1200,w=360,h=44` | no | no |
| `Y` | `floating_stone_y_1` | `x=760,y=760,w=300,h=40` | no | no |

Players falling from above can land on platforms. Set `solidSides: true` on a platform to make its left and right sides block horizontal movement. Set `solidCeiling: true` to make the platform bottom block upward head movement.

Walls:

| Room | Wall | Area |
| --- | --- | --- |
| `X` | `stone_wall_x_1` | `x=1500,y=1220,w=80,h=280` |
| `Y` | `stone_wall_y_1` | `x=1240,y=900,w=70,h=250` |

Walls block horizontal movement when the player's rectangle crosses the wall from either side.

Portals:

| From | Area | To | Target |
| --- | --- | --- | --- |
| `X` | `x=2350,y=1300,w=50,h=240` | `Y` | `x=120,y=1150` |
| `Y` | `x=0,y=980,w=60,h=220` | `X` | `x=2200,y=1500` |

Standing inside a portal area does not transfer automatically. The client must send `{"type":"portal"}` while inside the portal area; the server then transfers the player to the target room and sends that player a fresh `snapshot`. World events are also published to JetStream subjects under `battle.events.world.*` when JetStream is available.

## Message Subjects

The initial stream captures all subjects under:

```text
battle.events.>
```

Recommended subject examples:

```text
battle.events.match.created
battle.events.room.joined
battle.events.round.finished
battle.events.world.player_joined
battle.events.world.player_moved
battle.events.world.player_left
```

