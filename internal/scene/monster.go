package scene

import (
	"fmt"
	"strconv"
	"time"

	"mxd-battle/internal/combat"
	"mxd-battle/internal/world"
)

const (
	DefaultMonsterWidth  = 72
	DefaultMonsterHeight = 56
)

type Monster struct {
	ID            string              `json:"id"`
	MonsterID     string              `json:"monsterId"`
	Name          string              `json:"name"`
	X             float64             `json:"x"`
	Y             float64             `json:"y"`
	SpawnX        float64             `json:"spawnX"`
	SpawnY        float64             `json:"spawnY"`
	HomeMinX      float64             `json:"-"`
	HomeMaxX      float64             `json:"-"`
	Width         float64             `json:"width"`
	Height        float64             `json:"height"`
	MoveSpeed     float64             `json:"moveSpeed"`
	AggroRange    float64             `json:"aggroRange"`
	AggroTargetID string              `json:"aggroTargetId,omitempty"`
	AggroUntil    time.Time           `json:"aggroUntil,omitempty"`
	AttackRange   float64             `json:"attackRange"`
	AttackHeight  float64             `json:"attackHeight"`
	FacingX       float64             `json:"facingX"`
	HP            int32               `json:"hp"`
	HPMax         int32               `json:"hpMax"`
	ExpReward     int32               `json:"expReward"`
	CombatStat    combat.SnapshotStat `json:"combatStat"`
	Alive         bool                `json:"alive"`
	RespawnMS     int32               `json:"respawnMs"`
	RespawnAt     time.Time           `json:"respawnAt,omitempty"`
	LastAttackAt  time.Time           `json:"-"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

func spawnMonster(spawn world.MonsterSpawn, config combat.MonsterStatConfig, room *room, now time.Time) Monster {
	monster := Monster{
		ID:           spawnMonsterID(spawn, config),
		MonsterID:    config.ID,
		Name:         config.Name,
		X:            spawn.X,
		Y:            spawn.Y,
		SpawnX:       spawn.X,
		SpawnY:       spawn.Y,
		Width:        config.Width,
		Height:       config.Height,
		MoveSpeed:    config.MoveSpeed,
		AggroRange:   config.AggroRange,
		AttackRange:  config.AttackRange,
		AttackHeight: config.AttackHeight,
		FacingX:      -1,
		HP:           config.HPMax,
		HPMax:        config.HPMax,
		ExpReward:    config.ExpReward,
		CombatStat:   config.CombatStat,
		Alive:        true,
		RespawnMS:    firstNonZeroInt32(spawn.RespawnMS, config.RespawnMS),
		UpdatedAt:    now,
	}
	monster = normalizeMonsterBody(monster)
	monster = applyGroundToMonster(room.mapDef, monster, monster.Y, time.Time{})
	monster = bindMonsterHomePlatform(room.mapDef, monster)
	return monster
}

func spawnMonsterID(spawn world.MonsterSpawn, config combat.MonsterStatConfig) string {
	if spawn.ID != "" {
		return spawn.ID
	}
	return fmt.Sprintf("%s_spawn", config.ID)
}

func normalizeMonsterBody(monster Monster) Monster {
	if monster.Width <= 0 {
		monster.Width = DefaultMonsterWidth
	}
	if monster.Height <= 0 {
		monster.Height = DefaultMonsterHeight
	}
	if monster.FacingX == 0 {
		monster.FacingX = 1
	}
	if monster.HPMax <= 0 {
		monster.HPMax = 1
	}
	if monster.HP > monster.HPMax {
		monster.HP = monster.HPMax
	}
	if monster.AttackRange <= 0 {
		monster.AttackRange = 72
	}
	if monster.AttackHeight <= 0 {
		monster.AttackHeight = 72
	}
	return monster
}

func MonsterHalfWidth(monster Monster) float64 {
	return monster.Width / 2
}

func MonsterLeft(monster Monster) float64 {
	return monster.X - MonsterHalfWidth(monster)
}

func MonsterRight(monster Monster) float64 {
	return monster.X + MonsterHalfWidth(monster)
}

func MonsterTop(monster Monster) float64 {
	return monster.Y - monster.Height
}

func monsterRect(monster Monster) world.Rect {
	return world.Rect{X: MonsterLeft(monster), Y: MonsterTop(monster), Width: monster.Width, Height: monster.Height}
}

func monsterIntersectsRect(monster Monster, rect world.Rect) bool {
	return world.RectsOverlap(monsterRect(monster), rect)
}

func monsterCanHitPlayer(monster Monster, player Player) bool {
	return playerIntersectsRect(player, monsterAttackArea(monster)) ||
		world.RectsOverlap(monsterRect(monster), playerRect(player))
}

func monsterAttackArea(monster Monster) world.Rect {
	facingX := monster.FacingX
	if facingX == 0 {
		facingX = 1
	}
	height := monster.AttackHeight
	if height <= 0 {
		height = 72
	}
	rangeX := monster.AttackRange
	if rangeX <= 0 {
		rangeX = 72
	}
	y := monster.Y - monster.Height + (monster.Height-height)/2
	if facingX < 0 {
		return world.Rect{X: MonsterLeft(monster) - rangeX, Y: y, Width: rangeX, Height: height}
	}
	return world.Rect{X: MonsterRight(monster), Y: y, Width: rangeX, Height: height}
}

func applyGroundToMonster(mapDef world.MapConfig, monster Monster, previousY float64, now time.Time) Monster {
	return snapMonsterToGround(mapDef, monster)
}

func moveMonster(room *room, monster Monster, previousX float64, previousY float64, now time.Time) Monster {
	proxy := Player{X: monster.X, Y: monster.Y, Width: monster.Width, Height: monster.Height, FacingX: monster.FacingX, OnGround: true}
	proxy = NormalizePlayerBody(proxy)
	proxy.X = world.Clamp(proxy.X, PlayerHalfWidth(proxy), room.mapDef.Width-PlayerHalfWidth(proxy))
	proxy.X = resolveWallCollision(room.mapDef, Player{X: previousX, Y: previousY, Width: proxy.Width, Height: proxy.Height}, proxy)
	monster.X = clampMonsterToHomePlatform(proxy.X, monster)
	monster.Y = proxy.Y
	return snapMonsterToGround(room.mapDef, monster)
}

func snapMonsterToGround(mapDef world.MapConfig, monster Monster) Monster {
	monster.Y = monsterGroundY(mapDef, monster)
	return monster
}

func bindMonsterHomePlatform(mapDef world.MapConfig, monster Monster) Monster {
	for _, platform := range mapDef.Platforms {
		if !monsterOverlapsX(monster, platform.X, platform.Width) || !sameY(monster.Y, platform.Y) {
			continue
		}
		monster.HomeMinX = platform.X + MonsterHalfWidth(monster)
		monster.HomeMaxX = platform.X + platform.Width - MonsterHalfWidth(monster)
		if monster.HomeMinX > monster.HomeMaxX {
			center := platform.X + platform.Width/2
			monster.HomeMinX = center
			monster.HomeMaxX = center
		}
		monster.X = clampMonsterToHomePlatform(monster.X, monster)
		return monster
	}
	return monster
}

func clampMonsterToHomePlatform(x float64, monster Monster) float64 {
	if monster.HomeMinX == 0 && monster.HomeMaxX == 0 {
		return x
	}
	return world.Clamp(x, monster.HomeMinX, monster.HomeMaxX)
}

func sameY(a float64, b float64) bool {
	if a > b {
		return a-b <= wallSkin
	}
	return b-a <= wallSkin
}
func monsterGroundY(mapDef world.MapConfig, monster Monster) float64 {
	groundY := mapDef.GroundY
	proxy := Player{X: monster.X, Y: monster.Y, Width: monster.Width, Height: monster.Height}
	for _, x := range []float64{MonsterLeft(monster), monster.X, MonsterRight(monster)} {
		if y := terrainYAt(mapDef, x, groundY); monsterCanStandOnY(monster, y) && y < groundY {
			groundY = y
		}
	}
	for _, platform := range mapDef.Platforms {
		if monsterOverlapsX(monster, platform.X, platform.Width) && monsterCanStandOnY(monster, platform.Y) && platform.Y < groundY {
			groundY = platform.Y
		}
	}
	for _, wall := range mapDef.Walls {
		if monsterOverlapsX(monster, wall.X, wall.Width) && monsterCanStandOnY(monster, wall.Y) && wall.Y < groundY {
			groundY = wall.Y
		}
	}
	for _, polygon := range mapDef.Polygons {
		if y, ok := polygonLandingY(polygon, proxy); ok && monsterCanStandOnY(monster, y) && y < groundY {
			groundY = y
		}
	}
	return groundY
}

func monsterCanStandOnY(monster Monster, y float64) bool {
	return y >= monster.Y-maxWalkableTerrainStep
}

func monsterOverlapsX(monster Monster, x float64, width float64) bool {
	return MonsterRight(monster) >= x && MonsterLeft(monster) <= x+width
}

func respawnMonster(room *room, monster Monster, now time.Time) Monster {
	monster.Alive = true
	monster.HP = monster.HPMax
	monster.X = monster.SpawnX
	monster.Y = monster.SpawnY
	monster.RespawnAt = time.Time{}
	monster.LastAttackAt = time.Time{}
	monster.AggroTargetID = ""
	monster.AggroUntil = time.Time{}
	monster.UpdatedAt = now
	monster = applyGroundToMonster(room.mapDef, monster, monster.Y, time.Time{})
	return monster
}

func defeatMonster(monster Monster, now time.Time) Monster {
	monster.Alive = false
	monster.AggroTargetID = ""
	monster.AggroUntil = time.Time{}
	monster.HP = 0
	respawnMS := monster.RespawnMS
	if respawnMS <= 0 {
		respawnMS = 4000
	}
	monster.RespawnAt = now.Add(time.Duration(respawnMS) * time.Millisecond)
	monster.UpdatedAt = now
	return monster
}

func addExpString(current string, amount int32) string {
	if amount == 0 {
		if current == "" {
			return "0"
		}
		return current
	}
	value, err := strconv.ParseInt(current, 10, 64)
	if err != nil {
		value = 0
	}
	value += int64(amount)
	return strconv.FormatInt(value, 10)
}

func firstNonZeroInt32(values ...int32) int32 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
