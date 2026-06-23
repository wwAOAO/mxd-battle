package combat

import (
	"encoding/json"
	"fmt"
	"os"
)

type EquipmentConfigs map[string]EquipmentConfig

type EquipmentConfig struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Slot        string               `json:"slot"`
	Stat        BaseStat             `json:"stat"`
	Requirement EquipmentRequirement `json:"requirement"`
}

type EquipmentRequirement struct {
	Strength     int32 `json:"strength"`
	Intelligence int32 `json:"intelligence"`
	Agility      int32 `json:"agility"`
	Luck         int32 `json:"luck"`
}

func LoadEquipmentConfigs(path string) (EquipmentConfigs, error) {
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
	for id, config := range configs {
		if config.ID == "" {
			config.ID = id
		}
		if config.Slot == "" {
			config.Slot = "misc"
		}
		configs[id] = config
	}
	return configs, nil
}

func AggregateEquipmentStat(equipmentIDs []string, configs EquipmentConfigs) BaseStat {
	slotStats := make(map[string]BaseStat)
	slotOrder := make([]string, 0, len(equipmentIDs))
	seenSlots := make(map[string]struct{})
	for _, equipmentID := range equipmentIDs {
		config, ok := configs[equipmentID]
		if !ok {
			continue
		}
		slot := config.Slot
		if slot == "" {
			slot = "misc"
		}
		if _, ok := seenSlots[slot]; !ok {
			seenSlots[slot] = struct{}{}
			slotOrder = append(slotOrder, slot)
		}
		slotStats[slot] = config.Stat
	}

	var total BaseStat
	for _, slot := range slotOrder {
		total = addBaseStats(total, slotStats[slot])
	}
	return total
}

func MeetsEquipmentRequirement(stat BaseStat, cfg EquipmentConfig) bool {
	requirement := cfg.Requirement
	return stat.Strength >= requirement.Strength &&
		stat.Intelligence >= requirement.Intelligence &&
		stat.Agility >= requirement.Agility &&
		stat.Luck >= requirement.Luck
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
