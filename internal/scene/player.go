package scene

import (
	"time"

	"mxd-battle/internal/combat"
)

const (
	DefaultPlayerWidth        = 52
	DefaultPlayerHeight       = 96
	DefaultMaxPlayerMoveSpeed = 600
)

type Player struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Level             int32                `json:"level"`
	Exp               string               `json:"exp"`
	JobCode           string               `json:"jobCode"`
	Stat              PlayerStatBundle     `json:"stat"`
	CombatStat        combat.SnapshotStat  `json:"combatStat"`
	EquipmentIDs      []string             `json:"equipmentIds,omitempty"`
	X                 float64              `json:"x"`
	Y                 float64              `json:"y"`
	Width             float64              `json:"width"`
	Height            float64              `json:"height"`
	VX                float64              `json:"vx"`
	VY                float64              `json:"vy"`
	InputX            float64              `json:"inputX"`
	FacingX           float64              `json:"facingX"`
	OnGround          bool                 `json:"onGround"`
	DropUntil         time.Time            `json:"dropUntil,omitempty"`
	LastAttackAt      time.Time            `json:"lastAttackAt,omitempty"`
	LastSkillAt       map[string]time.Time `json:"lastSkillAt,omitempty"`
	ActionKind        string               `json:"actionKind,omitempty"`
	ActionLockedUntil time.Time            `json:"actionLockedUntil,omitempty"`
	RecoveryElapsedMS int64                `json:"-"`
	HPRecoveryCarry   float64              `json:"-"`
	MPRecoveryCarry   float64              `json:"-"`
	Room              string               `json:"room"`
	MapID             string               `json:"mapId"`
	UpdatedAt         time.Time            `json:"updatedAt"`
}

type PlayerStatBundle struct {
	Base      PlayerStat `json:"base"`
	Bonus     PlayerStat `json:"bonus"`
	Equipment PlayerStat `json:"equipment"`
	Extra     PlayerStat `json:"extra"`
	Final     PlayerStat `json:"final"`
}

type PlayerStat = combat.BaseStat

func DefaultPlayerStat() PlayerStat {
	return PlayerStat{
		HP:    500,
		MP:    100,
		HPMax: 500,
		MPMax: 100,
	}
}

func DefaultPlayerStatBundle() PlayerStatBundle {
	base := DefaultPlayerStat()
	return PlayerStatBundle{
		Base:  base,
		Final: base,
	}
}

func ApplyEquipmentStats(player Player, equipmentConfigs combat.EquipmentConfigs) Player {
	if len(equipmentConfigs) == 0 {
		player.Stat.Equipment = PlayerStat{}
		return player
	}
	player.Stat.Equipment = combat.AggregateEquipmentStat(player.EquipmentIDs, equipmentConfigs)
	return player
}

func EffectiveEquipmentRequirementStat(player Player) combat.BaseStat {
	return addStatLayers(player.Stat.Base, player.Stat.Bonus, player.Stat.Extra)
}

func FilterEquipmentsByRequirement(player Player, equipmentIDs []string, equipmentConfigs combat.EquipmentConfigs) []string {
	if len(equipmentIDs) == 0 || len(equipmentConfigs) == 0 {
		return nil
	}

	stat := EffectiveEquipmentRequirementStat(player)
	filtered := make([]string, 0, len(equipmentIDs))
	usedSlots := make(map[string]int)
	for _, equipmentID := range equipmentIDs {
		config, ok := equipmentConfigs[equipmentID]
		if !ok {
			continue
		}
		if !combat.MeetsEquipmentRequirement(stat, config) {
			continue
		}
		slot := config.Slot
		if slot == "" {
			slot = "misc"
		}
		if index, ok := usedSlots[slot]; ok {
			filtered[index] = equipmentID
			continue
		}
		usedSlots[slot] = len(filtered)
		filtered = append(filtered, equipmentID)
	}
	return filtered
}

func NormalizePlayerBody(player Player) Player {
	if player.Width <= 0 {
		player.Width = DefaultPlayerWidth
	}
	if player.Height <= 0 {
		player.Height = DefaultPlayerHeight
	}
	return player
}

func NormalizePlayerStat(player Player) Player {
	player.Stat.Base = normalizeStatLayer(player.Stat.Base)
	player.Stat.Bonus = normalizeAdditiveStatLayer(player.Stat.Bonus)
	player.Stat.Equipment = normalizeAdditiveStatLayer(player.Stat.Equipment)
	player.Stat.Extra = normalizeAdditiveStatLayer(player.Stat.Extra)
	player.Stat.Final = addStatLayers(player.Stat.Base, player.Stat.Bonus, player.Stat.Equipment, player.Stat.Extra)
	player.Stat.Final = normalizeStatLayer(player.Stat.Final)
	return player
}

func NormalizePlayerStatWithJobs(player Player, jobs combat.JobStatConfigs) Player {
	player = NormalizePlayerStat(player)
	player.JobCode, player.Stat.Final = combat.CalculateStatGrowth(player.Stat.Final, player.JobCode, jobs)
	player.Stat.Final = normalizeStatLayer(player.Stat.Final)
	return player
}
func normalizeStatLayer(stat PlayerStat) PlayerStat {
	defaultStat := DefaultPlayerStat()
	if stat.HPMax <= 0 {
		stat.HPMax = defaultStat.HPMax
	}
	if stat.MPMax <= 0 {
		stat.MPMax = defaultStat.MPMax
	}
	if stat.HP <= 0 {
		stat.HP = stat.HPMax
	}
	if stat.MP <= 0 {
		stat.MP = stat.MPMax
	}
	return stat
}

func normalizeAdditiveStatLayer(stat PlayerStat) PlayerStat {
	return stat
}

func addStatLayers(layers ...PlayerStat) PlayerStat {
	var total PlayerStat
	for _, layer := range layers {
		total.Strength += layer.Strength
		total.Intelligence += layer.Intelligence
		total.Agility += layer.Agility
		total.Luck += layer.Luck
		total.HP += layer.HP
		total.MP += layer.MP
		total.HPMax += layer.HPMax
		total.MPMax += layer.MPMax
	}
	return total
}

func PlayerHalfWidth(player Player) float64 {
	return player.Width / 2
}

func PlayerLeft(player Player) float64 {
	return player.X - PlayerHalfWidth(player)
}

func PlayerRight(player Player) float64 {
	return player.X + PlayerHalfWidth(player)
}

func PlayerTop(player Player) float64 {
	return player.Y - player.Height
}
