package combat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MonsterStatConfigs map[string]MonsterStatConfig

type MonsterStatConfig struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Width            float64      `json:"width"`
	Height           float64      `json:"height"`
	HPMax            int32        `json:"hpMax"`
	ExpReward        int32        `json:"expReward"`
	MoveSpeed        float64      `json:"moveSpeed"`
	AggroRange       float64      `json:"aggroRange"`
	AttackRange      float64      `json:"attackRange"`
	AttackHeight     float64      `json:"attackHeight"`
	RespawnMS        int32        `json:"respawnMs"`
	AttackIntervalMS int32        `json:"attackIntervalMs"`
	CombatStat       SnapshotStat `json:"combatStat"`
}

func LoadMonsterStatConfigs(path string) (MonsterStatConfigs, error) {
	configs, err := loadMonsterStatConfigFiles(path)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("monster stat config must define monsters")
	}
	return normalizeMonsterStatConfigs(configs), nil
}

func loadMonsterStatConfigFiles(path string) (MonsterStatConfigs, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat monster stat config: %w", err)
	}
	configs := make(MonsterStatConfigs)
	if !info.IsDir() {
		if err := loadMonsterStatConfigFile(path, configs); err != nil {
			return nil, err
		}
		return configs, nil
	}

	if err := filepath.WalkDir(path, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(filePath) != ".json" {
			return nil
		}
		return loadMonsterStatConfigFile(filePath, configs)
	}); err != nil {
		return nil, fmt.Errorf("read monster stat config directory: %w", err)
	}
	return configs, nil
}

func loadMonsterStatConfigFile(path string, configs MonsterStatConfigs) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read monster stat config %s: %w", path, err)
	}

	var group MonsterStatConfigs
	if err := json.Unmarshal(payload, &group); err == nil {
		for id, config := range group {
			configs[id] = config
		}
		return nil
	}

	var config MonsterStatConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return fmt.Errorf("decode monster stat config %s: %w", path, err)
	}
	id := config.ID
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		config.ID = id
	}
	configs[id] = config
	return nil
}

func normalizeMonsterStatConfigs(configs MonsterStatConfigs) MonsterStatConfigs {
	for id, config := range configs {
		if config.ID == "" {
			config.ID = id
		}
		if config.Width <= 0 {
			config.Width = 72
		}
		if config.Height <= 0 {
			config.Height = 72
		}
		if config.HPMax <= 0 {
			config.HPMax = 100
		}
		if config.MoveSpeed <= 0 {
			config.MoveSpeed = 120
		}
		if config.AggroRange <= 0 {
			config.AggroRange = 320
		}
		if config.AttackRange <= 0 {
			config.AttackRange = 72
		}
		if config.AttackHeight <= 0 {
			config.AttackHeight = 72
		}
		if config.RespawnMS <= 0 {
			config.RespawnMS = 4000
		}
		if config.AttackIntervalMS <= 0 {
			config.AttackIntervalMS = 1200
		}
		if config.CombatStat.AttackIntervalMS <= 0 {
			config.CombatStat.AttackIntervalMS = config.AttackIntervalMS
		}
		configs[id] = config
	}
	return configs
}
