package combat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type EquipmentConfigs map[string]EquipmentConfig

type EquipmentConfig struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Slot             string               `json:"slot"`
	SlotCount        int32                `json:"slotCount"`
	OccupiesSlots    []string             `json:"occupiesSlots"`
	Stat             BaseStat             `json:"stat"`
	CombatStat       EquipmentCombatStat  `json:"combatStat"`
	Requirement      EquipmentRequirement `json:"requirement"`
	MinLevel         int32                `json:"minLevel"`
	AllowedJobs      []string             `json:"allowedJobs"`
	AllowedGenders   []string             `json:"allowedGenders"`
	ExclusiveGroup   string               `json:"exclusiveGroup"`
	IncompatibleWith []string             `json:"incompatibleWith"`
}

type EquipmentCombatStat struct {
	PhysicalAttackMin int32   `json:"physicalAttackMin"`
	PhysicalAttackMax int32   `json:"physicalAttackMax"`
	MagicAttackMin    int32   `json:"magicAttackMin"`
	MagicAttackMax    int32   `json:"magicAttackMax"`
	PhysicalDefense   int32   `json:"physicalDefense"`
	MagicDefense      int32   `json:"magicDefense"`
	MoveSpeed         int32   `json:"moveSpeed"`
	Evasion           int32   `json:"evasion"`
	Accuracy          int32   `json:"accuracy"`
	CritRate          int32   `json:"critRate"`
	CritDamage        int32   `json:"critDamage"`
	HPRecovery        float64 `json:"hpRecovery"`
	MPRecovery        float64 `json:"mpRecovery"`
	CastSpeed         int32   `json:"castSpeed"`
	AttackStartupMS   int32   `json:"attackStartupMs"`
	AttackActiveMS    int32   `json:"attackActiveMs"`
	AttackRecoveryMS  int32   `json:"attackRecoveryMs"`
	AttackIntervalMS  int32   `json:"attackIntervalMs"`
}

type EquipmentRequirement struct {
	Strength     int32 `json:"strength"`
	Intelligence int32 `json:"intelligence"`
	Agility      int32 `json:"agility"`
	Luck         int32 `json:"luck"`
}

type EquipFailureReason struct {
	EquipmentID string `json:"equipmentId"`
	Slot        string `json:"slot,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

func NormalizeEquipmentConfigs(configs EquipmentConfigs) EquipmentConfigs {
	if len(configs) == 0 {
		return configs
	}
	normalized := make(EquipmentConfigs, len(configs))
	for id, config := range configs {
		if config.ID == "" {
			config.ID = id
		}
		if config.Slot == "" {
			config.Slot = "misc"
		}
		if config.SlotCount <= 0 {
			config.SlotCount = 1
		}
		if len(config.OccupiesSlots) == 0 {
			config.OccupiesSlots = defaultOccupiedSlots(config)
		}
		normalized[id] = config
	}
	return normalized
}

func LoadEquipmentConfigs(path string) (EquipmentConfigs, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat equipment config: %w", err)
	}
	if info.IsDir() {
		return loadEquipmentConfigsDir(path)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read equipment config: %w", err)
	}

	var configs EquipmentConfigs
	if err := json.Unmarshal(payload, &configs); err != nil {
		return nil, fmt.Errorf("decode equipment config: %w", err)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("equipment config must define equipments")
	}
	return NormalizeEquipmentConfigs(configs), nil
}

func loadEquipmentConfigsDir(dir string) (EquipmentConfigs, error) {
	jsonFiles, err := collectEquipmentJSONFiles(dir)
	if err != nil {
		return nil, err
	}
	if len(jsonFiles) == 0 {
		return nil, fmt.Errorf("equipment config dir must contain json files")
	}

	merged := make(EquipmentConfigs)
	for _, filePath := range jsonFiles {
		payload, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read equipment config %s: %w", filePath, err)
		}

		var partial EquipmentConfigs
		if err := json.Unmarshal(payload, &partial); err != nil {
			return nil, fmt.Errorf("decode equipment config %s: %w", filePath, err)
		}

		for id, cfg := range partial {
			if _, exists := merged[id]; exists {
				return nil, fmt.Errorf("duplicate equipment id %q in %s", id, filePath)
			}
			merged[id] = cfg
		}
	}

	if len(merged) == 0 {
		return nil, fmt.Errorf("equipment config must define equipments")
	}
	return NormalizeEquipmentConfigs(merged), nil
}

func collectEquipmentJSONFiles(dir string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk equipment config dir: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func AggregateEquipmentStat(equipmentIDs []string, configs EquipmentConfigs) BaseStat {
	var total BaseStat
	seen := make(map[string]struct{}, len(equipmentIDs))
	for _, equipmentID := range equipmentIDs {
		if _, ok := seen[equipmentID]; ok {
			continue
		}
		seen[equipmentID] = struct{}{}
		config, ok := configs[equipmentID]
		if !ok {
			continue
		}
		total = addBaseStats(total, config.Stat)
	}
	return total
}

func AggregateEquipmentCombatStat(equipmentIDs []string, configs EquipmentConfigs) EquipmentCombatStat {
	var total EquipmentCombatStat
	seen := make(map[string]struct{}, len(equipmentIDs))
	for _, equipmentID := range equipmentIDs {
		if _, ok := seen[equipmentID]; ok {
			continue
		}
		seen[equipmentID] = struct{}{}
		config, ok := configs[equipmentID]
		if !ok {
			continue
		}
		total = addEquipmentCombatStats(total, config.CombatStat)
	}
	return total
}

func ApplyEquipmentCombatStat(snapshot SnapshotStat, bonus EquipmentCombatStat) SnapshotStat {
	snapshot.PhysicalAttackMin += bonus.PhysicalAttackMin
	snapshot.PhysicalAttackMax += bonus.PhysicalAttackMax
	snapshot.MagicAttackMin += bonus.MagicAttackMin
	snapshot.MagicAttackMax += bonus.MagicAttackMax
	snapshot.PhysicalDefense += bonus.PhysicalDefense
	snapshot.MagicDefense += bonus.MagicDefense
	snapshot.MoveSpeed = clampInt32(snapshot.MoveSpeed+bonus.MoveSpeed, 0, 600)
	snapshot.Evasion += bonus.Evasion
	snapshot.Accuracy += bonus.Accuracy
	snapshot.CritRate += bonus.CritRate
	snapshot.CritDamage += bonus.CritDamage
	snapshot.HPRecovery += bonus.HPRecovery
	snapshot.MPRecovery += bonus.MPRecovery
	snapshot.CastSpeed += bonus.CastSpeed
	snapshot.AttackStartupMS = maxInt32(0, snapshot.AttackStartupMS+bonus.AttackStartupMS)
	snapshot.AttackActiveMS = maxInt32(0, snapshot.AttackActiveMS+bonus.AttackActiveMS)
	snapshot.AttackRecoveryMS = maxInt32(0, snapshot.AttackRecoveryMS+bonus.AttackRecoveryMS)
	snapshot.AttackIntervalMS = maxInt32(0, snapshot.AttackIntervalMS+bonus.AttackIntervalMS)
	return snapshot
}

func MeetsEquipmentRequirement(stat BaseStat, level int32, jobCode string, gender string, cfg EquipmentConfig) bool {
	requirement := cfg.Requirement
	if stat.Strength < requirement.Strength ||
		stat.Intelligence < requirement.Intelligence ||
		stat.Agility < requirement.Agility ||
		stat.Luck < requirement.Luck {
		return false
	}
	if cfg.MinLevel > 0 && level < cfg.MinLevel {
		return false
	}
	if !matchesAllowedValue(jobCode, cfg.AllowedJobs) {
		return false
	}
	if !matchesAllowedValue(gender, cfg.AllowedGenders) {
		return false
	}
	return true
}

func matchesAllowedValue(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func defaultOccupiedSlots(config EquipmentConfig) []string {
	baseSlot := strings.TrimSpace(config.Slot)
	if baseSlot == "" {
		baseSlot = "misc"
	}
	slotCount := config.SlotCount
	if slotCount <= 0 {
		slotCount = 1
	}

	switch baseSlot {
	case "ring":
		if slotCount >= 2 {
			return []string{"ring1", "ring2"}
		}
		return []string{"ring1"}
	case "weapon_main":
		if slotCount >= 2 {
			return []string{"weapon_main", "weapon_sub"}
		}
		return []string{"weapon_main"}
	case "weapon_sub":
		return []string{"weapon_sub"}
	default:
		return []string{baseSlot}
	}
}

func addBaseStats(left BaseStat, right BaseStat) BaseStat {
	left.Strength += right.Strength
	left.Intelligence += right.Intelligence
	left.Agility += right.Agility
	left.Luck += right.Luck
	left.HP += right.HP
	left.MP += right.MP
	left.HPMax += right.HPMax
	left.MPMax += right.MPMax
	return left
}

func addEquipmentCombatStats(left EquipmentCombatStat, right EquipmentCombatStat) EquipmentCombatStat {
	left.PhysicalAttackMin += right.PhysicalAttackMin
	left.PhysicalAttackMax += right.PhysicalAttackMax
	left.MagicAttackMin += right.MagicAttackMin
	left.MagicAttackMax += right.MagicAttackMax
	left.PhysicalDefense += right.PhysicalDefense
	left.MagicDefense += right.MagicDefense
	left.MoveSpeed += right.MoveSpeed
	left.Evasion += right.Evasion
	left.Accuracy += right.Accuracy
	left.CritRate += right.CritRate
	left.CritDamage += right.CritDamage
	left.HPRecovery += right.HPRecovery
	left.MPRecovery += right.MPRecovery
	left.CastSpeed += right.CastSpeed
	left.AttackStartupMS += right.AttackStartupMS
	left.AttackActiveMS += right.AttackActiveMS
	left.AttackRecoveryMS += right.AttackRecoveryMS
	left.AttackIntervalMS += right.AttackIntervalMS
	return left
}

func maxInt32(a int32, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
