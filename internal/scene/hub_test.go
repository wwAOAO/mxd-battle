package scene

import (
	"testing"
	"time"

	"mxd-battle/internal/combat"
	"mxd-battle/internal/world"
)

type recordingPeer struct {
	events []ServerEvent
}

func (p *recordingPeer) Send(event ServerEvent) {
	p.events = append(p.events, event)
}

func newTestHub(t *testing.T) *Hub {
	t.Helper()

	maps := testRoomMaps()
	hub, err := NewHub(nil, nil, maps)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	return hub
}

func newMonsterTestHub(t *testing.T) *Hub {
	t.Helper()
	maps := map[string]world.MapConfig{
		RoomX: {
			ID:           "monster_test",
			Width:        1800,
			Height:       1500,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			MonsterSpawns: []world.MonsterSpawn{
				{ID: "slime_1", MonsterID: "green_slime", X: 420, Y: 1500, RespawnMS: 500},
			},
		},
	}
	jobs := combat.JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 100},
			},
		},
	}
	monsters := combat.MonsterStatConfigs{
		"green_slime": {
			ID:               "green_slime",
			Name:             "Green Slime",
			Width:            60,
			Height:           48,
			HPMax:            20,
			ExpReward:        15,
			MoveSpeed:        0,
			AggroRange:       200,
			AttackRange:      72,
			AttackHeight:     60,
			RespawnMS:        500,
			AttackIntervalMS: 300,
			CombatStat: combat.SnapshotStat{
				PhysicalAttackMin: 8,
				PhysicalAttackMax: 12,
				PhysicalDefense:   0,
				MagicDefense:      0,
				AttackIntervalMS:  300,
			},
		},
	}
	hub, err := NewHubWithJobsEquipmentAndMonsters(nil, nil, maps, jobs, nil, monsters)
	if err != nil {
		t.Fatalf("new monster hub: %v", err)
	}
	return hub
}

func testRoomMaps() map[string]world.MapConfig {
	return map[string]world.MapConfig{
		RoomX: {
			ID:           "wild_x",
			Name:         "Wild Map X",
			Width:        3200,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain:      []world.Terrain{{ID: "ground_x_1", X1: 0, Y1: 1500, X2: 700, Y2: 1500}, {ID: "slope_x_1", X1: 700, Y1: 1500, X2: 1100, Y2: 1400}, {ID: "ground_x_2", X1: 1100, Y1: 1400, X2: 3200, Y2: 1400}},
			Platforms:    []world.Platform{{ID: "floating_stone_x_1", X: 900, Y: 1350, Width: 360, Height: 44}, {ID: "floating_stone_x_2", X: 900, Y: 1250, Width: 360, Height: 44}, {ID: "floating_stone_x_3", X: 1600, Y: 1400, Width: 360, Height: 44}, {ID: "floating_stone_x_4", X: 1700, Y: 1300, Width: 360, Height: 44}, {ID: "floating_stone_x_5", X: 1820, Y: 1200, Width: 360, Height: 44, SolidSides: true, SolidCeiling: true}},
			Walls:        []world.Wall{{ID: "stone_wall_x_1", X: 1500, Y: 1220, Width: 80, Height: 280}},
			Portals:      []world.Portal{{ID: "x_to_y", TargetRoomID: RoomY, Area: world.Rect{X: 3140, Y: 1300, Width: 60, Height: 240}, Target: world.Point{X: 120, Y: 1150}}},
		},
		RoomY: {
			ID:           "wild_y",
			Name:         "Wild Map Y",
			Width:        2600,
			Height:       1400,
			GroundY:      1150,
			Gravity:      2600,
			JumpVelocity: -920,
			MoveSpeed:    400,
			Spawn:        world.Point{X: 120, Y: 1150},
			Portals:      []world.Portal{},
		},
	}
}

func TestHubMonsterCanBeDefeatedAndRespawns(t *testing.T) {
	hub := newMonsterTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "hero", X: 340, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 25, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join hero: %v", err)
	}

	result, _, ok := hub.NormalAttack("hero")
	if !ok {
		t.Fatal("expected normal attack to succeed")
	}
	if result.TargetType != "monster" || result.TargetID != "slime_1" {
		t.Fatalf("expected attack to hit slime_1 monster, got %+v", result)
	}
	if !result.DefeatedTarget || result.ExpReward != 15 {
		t.Fatalf("expected defeated monster exp reward, got %+v", result)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after attack: %v", err)
	}
	monster := state.Monsters["slime_1"]
	if monster.Alive || monster.HP != 0 {
		t.Fatalf("expected monster to be dead, got %+v", monster)
	}
	player := state.Players["hero"]
	if player.Exp != "15" {
		t.Fatalf("expected hero exp 15, got %+v", player)
	}

	hub.StepPhysics(monster.RespawnAt.Add(50*time.Millisecond), 0.05)
	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after respawn: %v", err)
	}
	monster = state.Monsters["slime_1"]
	if !monster.Alive || monster.HP != monster.HPMax {
		t.Fatalf("expected monster to respawn with full hp, got %+v", monster)
	}
}

func TestHubPlayerCanPickupMonsterLoot(t *testing.T) {
	hub := newMonsterTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "hero", X: 340, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 25, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join hero: %v", err)
	}

	if _, _, ok := hub.NormalAttack("hero"); !ok {
		t.Fatal("expected normal attack to succeed")
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after attack: %v", err)
	}
	if len(state.LootDrops) != 1 {
		t.Fatalf("expected one loot drop, got %d", len(state.LootDrops))
	}

	player, loot, ok := hub.PickupLoot("hero")
	if !ok {
		t.Fatal("expected pickup to succeed")
	}
	if loot.ID == "" {
		t.Fatalf("expected picked loot id, got %+v", loot)
	}
	if player.ID != "hero" {
		t.Fatalf("expected hero pickup result, got %+v", player)
	}

	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after pickup: %v", err)
	}
	if len(state.LootDrops) != 0 {
		t.Fatalf("expected loot to be removed after pickup, got %+v", state.LootDrops)
	}
}
func TestHubMonsterAttacksPlayer(t *testing.T) {
	hub := newMonsterTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "hero", X: 455, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 10, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join hero: %v", err)
	}

	hub.StepPhysics(testTime().Add(350*time.Millisecond), 0.05)
	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after monster attack: %v", err)
	}
	player := state.Players["hero"]
	if player.Stat.Final.HP >= 500 {
		t.Fatalf("expected monster to damage hero, got %+v", player)
	}
}

func TestMonsterAggroTargetsPlayerWhoAttackedIt(t *testing.T) {
	hub := newMonsterTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "near", X: 560, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 5, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join near: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "attacker", X: 330, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 5, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join attacker: %v", err)
	}

	monster := hub.rooms[RoomX].monsters["slime_1"]
	monster.HP = 200
	monster.HPMax = 200
	hub.rooms[RoomX].monsters["slime_1"] = monster

	result, _, ok := hub.normalAttackAt("attacker", testTime())
	if !ok || result.TargetType != "monster" {
		t.Fatalf("expected attacker to hit monster, ok=%v result=%+v", ok, result)
	}

	monster = hub.rooms[RoomX].monsters["slime_1"]
	if monster.AggroTargetID != "attacker" {
		t.Fatalf("expected monster aggro target attacker, got %+v", monster)
	}
	target, ok := nearestMonsterTarget(monster, hub.rooms[RoomX].players, testTime())
	if !ok || target.ID != "attacker" {
		t.Fatalf("expected monster to prefer aggro attacker, ok=%v target=%+v", ok, target)
	}
}

func TestMonsterAggroExpiresAfterChasingForAWhile(t *testing.T) {
	hub := newMonsterTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "near", X: 500, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 5, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join near: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "attacker", X: 330, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 5, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join attacker: %v", err)
	}

	monster := hub.rooms[RoomX].monsters["slime_1"]
	monster.HP = 200
	monster.HPMax = 200
	hub.rooms[RoomX].monsters["slime_1"] = monster

	result, _, ok := hub.normalAttackAt("attacker", testTime())
	if !ok || result.TargetType != "monster" {
		t.Fatalf("expected attacker to hit monster, ok=%v result=%+v", ok, result)
	}

	hub.StepPhysics(testTime().Add(defaultMonsterAggroDuration+time.Millisecond), 0.05)
	monster = hub.rooms[RoomX].monsters["slime_1"]
	if monster.AggroTargetID != "" {
		t.Fatalf("expected monster aggro to expire, got %+v", monster)
	}
	target, ok := nearestMonsterTarget(monster, hub.rooms[RoomX].players, testTime().Add(defaultMonsterAggroDuration+time.Millisecond))
	if !ok || target.ID != "near" {
		t.Fatalf("expected monster to fall back to nearest target after aggro expires, ok=%v target=%+v", ok, target)
	}
}
func TestHubMonsterCanOverlapPlayerAndStillAttack(t *testing.T) {
	hub := newMonsterTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "hero", X: 520, Y: 1500, Stat: PlayerStatBundle{Base: PlayerStat{Strength: 10, HP: 500, MP: 100, HPMax: 500, MPMax: 100}}}, &recordingPeer{}); err != nil {
		t.Fatalf("join hero: %v", err)
	}

	monster := hub.rooms[RoomX].monsters["slime_1"]
	monster.MoveSpeed = 500
	monster.AttackRange = 1
	hub.rooms[RoomX].monsters["slime_1"] = monster

	hub.StepPhysics(testTime().Add(100*time.Millisecond), 0.2)
	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after monster move: %v", err)
	}
	player := state.Players["hero"]
	monster = state.Monsters["slime_1"]
	if !world.RectsOverlap(monsterRect(monster), playerRect(player)) {
		t.Fatalf("expected monster to overlap player without physical blocking, monster=%+v player=%+v", monster, player)
	}

	hub.StepPhysics(testTime().Add(450*time.Millisecond), 0.05)
	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after monster attack: %v", err)
	}
	player = state.Players["hero"]
	if player.Stat.Final.HP >= 500 {
		t.Fatalf("expected overlapping monster to damage player, got %+v", player)
	}
}

func TestMoveMonsterStaysOnHomePlatform(t *testing.T) {
	room := newRoom(RoomX, world.MapConfig{
		ID:           "monster_ground_test",
		Width:        900,
		Height:       600,
		GroundY:      500,
		Gravity:      2600,
		JumpVelocity: -980,
		MoveSpeed:    420,
		Spawn:        world.Point{X: 120, Y: 500},
		Platforms:    []world.Platform{{ID: "ledge", X: 100, Y: 360, Width: 160, Height: 40}},
	})
	monster := Monster{ID: "slime_1", X: 240, Y: 360, Width: 60, Height: 48, FacingX: 1}
	monster = bindMonsterHomePlatform(room.mapDef, monster)
	monster.X = 340
	monster = moveMonster(room, monster, 240, 360, testTime())
	if monster.X != 230 {
		t.Fatalf("expected monster to stay on platform right edge, got %+v", monster)
	}
	if monster.Y != 360 {
		t.Fatalf("expected monster to remain on platform, got %+v", monster)
	}
}

func TestReloadRoomsKicksOnlyPlayersInReloadedRooms(t *testing.T) {
	hub := newTestHub(t)
	xPeer := &recordingPeer{}
	yPeer := &recordingPeer{}
	if _, err := hub.Join(RoomX, Player{ID: "x_player"}, xPeer); err != nil {
		t.Fatalf("join x player: %v", err)
	}
	if _, err := hub.Join(RoomY, Player{ID: "y_player"}, yPeer); err != nil {
		t.Fatalf("join y player: %v", err)
	}

	maps := testRoomMaps()
	maps[RoomX] = world.MapConfig{
		ID:           "wild_x_reloaded",
		Name:         "Wild Map X Reloaded",
		Width:        1200,
		Height:       900,
		GroundY:      800,
		Gravity:      2600,
		JumpVelocity: -980,
		MoveSpeed:    420,
		Spawn:        world.Point{X: 80, Y: 800},
	}

	reloaded, kicked, err := hub.ReloadRooms(maps, []string{RoomX})
	if err != nil {
		t.Fatalf("reload rooms: %v", err)
	}
	if kicked != 1 || len(reloaded) != 1 || reloaded[0] != RoomX {
		t.Fatalf("expected only room x to reload and kick one player, reloaded=%v kicked=%d", reloaded, kicked)
	}
	if len(xPeer.events) == 0 || xPeer.events[len(xPeer.events)-1].Type != "room_reloaded" {
		t.Fatalf("expected x player to receive room_reloaded event, got %+v", xPeer.events)
	}
	if _, ok := hub.players["x_player"]; ok {
		t.Fatal("expected x player to be removed from hub")
	}
	stateX, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state x: %v", err)
	}
	if len(stateX.Players) != 0 || stateX.Map.ID != "wild_x_reloaded" {
		t.Fatalf("expected room x to be replaced with no players, got %+v", stateX)
	}
	stateY, err := hub.State(RoomY)
	if err != nil {
		t.Fatalf("state y: %v", err)
	}
	if _, ok := stateY.Players["y_player"]; !ok {
		t.Fatalf("expected y player to remain in unchanged room, got %+v", stateY.Players)
	}
}

func TestHubOnlyAllowsXYRooms(t *testing.T) {
	hub := newTestHub(t)

	_, err := hub.Join("Z", Player{ID: "p1"}, &recordingPeer{})
	if err != ErrInvalidRoom {
		t.Fatalf("expected ErrInvalidRoom, got %v", err)
	}
}

func testTime() time.Time {
	return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
}
