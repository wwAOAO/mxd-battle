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
	Gender            string               `json:"gender,omitempty"`
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
	InputY            float64              `json:"inputY"`
	FacingX           float64              `json:"facingX"`
	OnGround          bool                 `json:"onGround"`
	OnLadder          bool                 `json:"onLadder"`
	LadderID          string               `json:"ladderId,omitempty"`
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
	result := EvaluateEquipmentSelection(player, equipmentIDs, equipmentConfigs)
	return result.EquipmentIDs
}

type EquipmentSelectionResult struct {
	EquipmentIDs     []string                    `json:"equipmentIds"`
	EquipmentsBySlot map[string]string           `json:"equipmentsBySlot,omitempty"`
	Failures         []combat.EquipFailureReason `json:"failures,omitempty"`
}

func EvaluateEquipmentSelection(player Player, equipmentIDs []string, equipmentConfigs combat.EquipmentConfigs) EquipmentSelectionResult {
	if len(equipmentIDs) == 0 || len(equipmentConfigs) == 0 {
		return EquipmentSelectionResult{}
	}

	stat := EffectiveEquipmentRequirementStat(player)
	filtered := make([]string, 0, len(equipmentIDs))
	failures := make([]combat.EquipFailureReason, 0)
	occupiedSlots := make(map[string]string)
	usedGroups := make(map[string]string)
	selected := make(map[string]combat.EquipmentConfig)

	for _, equipmentID := range equipmentIDs {
		config, ok := equipmentConfigs[equipmentID]
		if !ok {
			failures = append(failures, combat.EquipFailureReason{
				EquipmentID: equipmentID,
				Code:        "not_found",
				Message:     "equipment is not configured",
			})
			continue
		}

		if failure, blocked := validateEquipmentRequirement(player, stat, config); blocked {
			failures = append(failures, failure)
			continue
		}
		if failure, blocked := validateEquipmentConflict(config, selected, usedGroups); blocked {
			failures = append(failures, failure)
			continue
		}
		assignedSlots := assignEquipmentSlots(config, occupiedSlots)
		if len(assignedSlots) == 0 {
			failure := combat.EquipFailureReason{
				EquipmentID: config.ID,
				Slot:        config.Slot,
				Code:        "slot_occupied",
				Message:     "required equipment slot is already occupied",
			}
			failures = append(failures, failure)
			continue
		}
		if failure, blocked := validateEquipmentSlots(config, assignedSlots, occupiedSlots); blocked {
			failures = append(failures, failure)
			continue
		}

		filtered = append(filtered, equipmentID)
		selected[equipmentID] = config
		for _, slot := range assignedSlots {
			occupiedSlots[slot] = equipmentID
		}
		if group := config.ExclusiveGroup; group != "" {
			usedGroups[group] = equipmentID
		}
	}

	return EquipmentSelectionResult{
		EquipmentIDs:     filtered,
		EquipmentsBySlot: occupiedSlots,
		Failures:         failures,
	}
}

func validateEquipmentRequirement(player Player, stat combat.BaseStat, config combat.EquipmentConfig) (combat.EquipFailureReason, bool) {
	requirement := config.Requirement
	if (stat.Strength < requirement.Strength) ||
		(stat.Intelligence < requirement.Intelligence) ||
		(stat.Agility < requirement.Agility) ||
		(stat.Luck < requirement.Luck) {
		return combat.EquipFailureReason{
			EquipmentID: config.ID,
			Slot:        config.Slot,
			Code:        "requirement_stat",
			Message:     "primary stat requirement is not met",
		}, true
	}
	if config.MinLevel > 0 && player.Level < config.MinLevel {
		return combat.EquipFailureReason{
			EquipmentID: config.ID,
			Slot:        config.Slot,
			Code:        "requirement_level",
			Message:     "level requirement is not met",
		}, true
	}
	if len(config.AllowedJobs) > 0 && !combat.MeetsEquipmentRequirement(stat, player.Level, player.JobCode, player.Gender, combat.EquipmentConfig{AllowedJobs: config.AllowedJobs}) {
		return combat.EquipFailureReason{
			EquipmentID: config.ID,
			Slot:        config.Slot,
			Code:        "requirement_job",
			Message:     "job restriction does not allow this equipment",
		}, true
	}
	if len(config.AllowedGenders) > 0 && !combat.MeetsEquipmentRequirement(stat, player.Level, player.JobCode, player.Gender, combat.EquipmentConfig{AllowedGenders: config.AllowedGenders}) {
		return combat.EquipFailureReason{
			EquipmentID: config.ID,
			Slot:        config.Slot,
			Code:        "requirement_gender",
			Message:     "gender restriction does not allow this equipment",
		}, true
	}
	return combat.EquipFailureReason{}, false
}

func validateEquipmentConflict(config combat.EquipmentConfig, selected map[string]combat.EquipmentConfig, usedGroups map[string]string) (combat.EquipFailureReason, bool) {
	for selectedID, selectedConfig := range selected {
		for _, blocked := range config.IncompatibleWith {
			if blocked == selectedID {
				return combat.EquipFailureReason{
					EquipmentID: config.ID,
					Slot:        config.Slot,
					Code:        "incompatible_with",
					Message:     "equipment conflicts with another equipped item",
				}, true
			}
		}
		for _, blocked := range selectedConfig.IncompatibleWith {
			if blocked == config.ID {
				return combat.EquipFailureReason{
					EquipmentID: config.ID,
					Slot:        config.Slot,
					Code:        "incompatible_with",
					Message:     "equipment conflicts with another equipped item",
				}, true
			}
		}
	}
	if group := config.ExclusiveGroup; group != "" {
		if otherID, exists := usedGroups[group]; exists {
			return combat.EquipFailureReason{
				EquipmentID: config.ID,
				Slot:        config.Slot,
				Code:        "exclusive_group",
				Message:     "equipment shares an exclusive group with " + otherID,
			}, true
		}
	}
	return combat.EquipFailureReason{}, false
}

func validateEquipmentSlots(config combat.EquipmentConfig, slots []string, occupiedSlots map[string]string) (combat.EquipFailureReason, bool) {
	for _, slot := range slots {
		if occupiedBy, exists := occupiedSlots[slot]; exists {
			return combat.EquipFailureReason{
				EquipmentID: config.ID,
				Slot:        slot,
				Code:        "slot_occupied",
				Message:     "slot is already occupied by " + occupiedBy,
			}, true
		}
	}
	return combat.EquipFailureReason{}, false
}

func assignEquipmentSlots(config combat.EquipmentConfig, occupiedSlots map[string]string) []string {
	if config.Slot == "ring" && len(config.OccupiesSlots) <= 1 {
		for _, slot := range []string{"ring1", "ring2"} {
			if _, exists := occupiedSlots[slot]; !exists {
				return []string{slot}
			}
		}
		return nil
	}
	for _, slot := range config.OccupiesSlots {
		if _, exists := occupiedSlots[slot]; exists {
			return nil
		}
	}
	return append([]string(nil), config.OccupiesSlots...)
}

func NormalizeEquipmentSelection(player Player, equipmentConfigs combat.EquipmentConfigs) (Player, EquipmentSelectionResult) {
	selection := EvaluateEquipmentSelection(player, player.EquipmentIDs, equipmentConfigs)
	player.EquipmentIDs = selection.EquipmentIDs
	return player, selection
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
