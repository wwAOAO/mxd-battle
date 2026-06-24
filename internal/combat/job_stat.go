package combat

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const DefaultJobCode = "beginner"

const (
	DefaultAttackIntervalMS    = 600
	DefaultMinAttackIntervalMS = 250
	DefaultMaxAttackIntervalMS = 1500
)

type JobStatConfigs map[string]JobStatConfig

type JobStatConfig struct {
	Name             string               `json:"name"`
	AttackIntervalMS int32                `json:"attackIntervalMs"`
	AttackInterval   AttackIntervalConfig `json:"attackInterval"`
	Allocation       CombatStatAllocation `json:"allocation"`
}

type AttackIntervalConfig struct {
	BaseMS     int32         `json:"baseMs"`
	MinMS      int32         `json:"minMs"`
	MaxMS      int32         `json:"maxMs"`
	StartupMS  int32         `json:"startupMs"`
	ActiveMS   int32         `json:"activeMs"`
	RecoveryMS int32         `json:"recoveryMs"`
	Reduction  StatReduction `json:"reduction"`
}

type StatReduction struct {
	Strength     float64 `json:"strength,omitempty"`
	Intelligence float64 `json:"intelligence,omitempty"`
	Agility      float64 `json:"agility,omitempty"`
	Luck         float64 `json:"luck,omitempty"`
}

type CombatStatAllocation struct {
	HPMax             StatPercent `json:"hpMax"`
	MPMax             StatPercent `json:"mpMax"`
	PhysicalAttackMin StatPercent `json:"physicalAttackMin"`
	PhysicalAttackMax StatPercent `json:"physicalAttackMax"`
	MagicAttackMin    StatPercent `json:"magicAttackMin"`
	MagicAttackMax    StatPercent `json:"magicAttackMax"`
	PhysicalDefense   StatPercent `json:"physicalDefense"`
	MagicDefense      StatPercent `json:"magicDefense"`
	MoveSpeed         StatPercent `json:"moveSpeed"`
	Evasion           StatPercent `json:"evasion"`
	Accuracy          StatPercent `json:"accuracy"`
	CritRate          StatPercent `json:"critRate"`
	CritDamage        StatPercent `json:"critDamage"`
	HPRecovery        StatPercent `json:"hpRecovery"`
	MPRecovery        StatPercent `json:"mpRecovery"`
}

type StatPercent struct {
	Strength     float64 `json:"strength,omitempty"`
	Intelligence float64 `json:"intelligence,omitempty"`
	Agility      float64 `json:"agility,omitempty"`
	Luck         float64 `json:"luck,omitempty"`
}

type BaseStat struct {
	Strength     int32 `json:"strength"`
	Intelligence int32 `json:"intelligence"`
	Agility      int32 `json:"agility"`
	Luck         int32 `json:"luck"`
	HP           int32 `json:"hp"`
	MP           int32 `json:"mp"`
	HPMax        int32 `json:"hpMax"`
	MPMax        int32 `json:"mpMax"`
}

type SnapshotStat struct {
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

func CalculateSnapshotStat(stat BaseStat, jobCode string, configs JobStatConfigs) (string, SnapshotStat) {
	if jobCode == "" {
		jobCode = DefaultJobCode
	}
	config, ok := configs[jobCode]
	if !ok {
		jobCode = DefaultJobCode
		config = configs[DefaultJobCode]
	}

	allocation := config.Allocation
	attackTiming := calculateAttackTiming(stat, config)
	return jobCode, SnapshotStat{
		PhysicalAttackMin: calculateAllocatedStat(stat, allocation.PhysicalAttackMin),
		PhysicalAttackMax: calculateAllocatedStat(stat, allocation.PhysicalAttackMax),
		MagicAttackMin:    calculateAllocatedStat(stat, allocation.MagicAttackMin),
		MagicAttackMax:    calculateAllocatedStat(stat, allocation.MagicAttackMax),
		PhysicalDefense:   calculateAllocatedStat(stat, allocation.PhysicalDefense),
		MagicDefense:      calculateAllocatedStat(stat, allocation.MagicDefense),
		MoveSpeed:         clampInt32(calculateAllocatedStat(stat, allocation.MoveSpeed), 0, 600),
		Evasion:           calculateAllocatedStat(stat, allocation.Evasion),
		Accuracy:          calculateAllocatedStat(stat, allocation.Accuracy),
		CritRate:          calculateAllocatedStat(stat, allocation.CritRate),
		CritDamage:        calculateAllocatedStat(stat, allocation.CritDamage),
		HPRecovery:        calculateAllocatedFloatStat(stat, allocation.HPRecovery),
		MPRecovery:        calculateAllocatedFloatStat(stat, allocation.MPRecovery),
		AttackStartupMS:   attackTiming.StartupMS,
		AttackActiveMS:    attackTiming.ActiveMS,
		AttackRecoveryMS:  attackTiming.RecoveryMS,
		AttackIntervalMS:  attackTiming.IntervalMS,
	}
}

func CalculateStatGrowth(stat BaseStat, jobCode string, configs JobStatConfigs) (string, BaseStat) {
	if jobCode == "" {
		jobCode = DefaultJobCode
	}
	config, ok := configs[jobCode]
	if !ok {
		jobCode = DefaultJobCode
		config = configs[DefaultJobCode]
	}

	allocation := config.Allocation
	stat.HPMax += calculateAllocatedStat(stat, allocation.HPMax)
	stat.MPMax += calculateAllocatedStat(stat, allocation.MPMax)
	if stat.HP > stat.HPMax {
		stat.HP = stat.HPMax
	}
	if stat.MP > stat.MPMax {
		stat.MP = stat.MPMax
	}
	return jobCode, stat
}

type attackTiming struct {
	StartupMS  int32
	ActiveMS   int32
	RecoveryMS int32
	IntervalMS int32
}

func calculateAttackTiming(stat BaseStat, config JobStatConfig) attackTiming {
	intervalMS := calculateAttackIntervalMS(stat, config)
	return splitAttackInterval(intervalMS, config.AttackInterval)
}

func calculateAttackIntervalMS(stat BaseStat, config JobStatConfig) int32 {
	interval := config.AttackInterval
	if interval.BaseMS <= 0 && interval.MinMS <= 0 && interval.MaxMS <= 0 &&
		interval.StartupMS <= 0 && interval.ActiveMS <= 0 && interval.RecoveryMS <= 0 {
		return fixedAttackIntervalMS(config)
	}

	baseMS := interval.BaseMS
	if baseMS <= 0 {
		baseMS = DefaultAttackIntervalMS
	}
	minMS := interval.MinMS
	if minMS <= 0 {
		minMS = DefaultMinAttackIntervalMS
	}
	maxMS := interval.MaxMS
	if maxMS <= 0 {
		maxMS = DefaultMaxAttackIntervalMS
	}
	if maxMS < minMS {
		maxMS = minMS
	}

	reduction := float64(stat.Strength)*interval.Reduction.Strength +
		float64(stat.Intelligence)*interval.Reduction.Intelligence +
		float64(stat.Agility)*interval.Reduction.Agility +
		float64(stat.Luck)*interval.Reduction.Luck
	value := int32(math.Round(float64(baseMS) - reduction))
	return clampInt32(value, minMS, maxMS)
}

func splitAttackInterval(intervalMS int32, config AttackIntervalConfig) attackTiming {
	startupMS := config.StartupMS
	activeMS := config.ActiveMS
	recoveryMS := config.RecoveryMS
	if startupMS <= 0 && activeMS <= 0 && recoveryMS <= 0 {
		startupMS = intervalMS / 4
		activeMS = intervalMS / 6
		recoveryMS = intervalMS - startupMS - activeMS
		return attackTiming{
			StartupMS:  startupMS,
			ActiveMS:   activeMS,
			RecoveryMS: recoveryMS,
			IntervalMS: intervalMS,
		}
	}

	baseTotal := startupMS + activeMS + recoveryMS
	if baseTotal <= 0 {
		return attackTiming{IntervalMS: intervalMS}
	}

	scaledStartup := int32(math.Round(float64(intervalMS) * float64(startupMS) / float64(baseTotal)))
	scaledActive := int32(math.Round(float64(intervalMS) * float64(activeMS) / float64(baseTotal)))
	scaledRecovery := intervalMS - scaledStartup - scaledActive
	if scaledRecovery < 0 {
		scaledRecovery = 0
	}

	return attackTiming{
		StartupMS:  scaledStartup,
		ActiveMS:   scaledActive,
		RecoveryMS: scaledRecovery,
		IntervalMS: intervalMS,
	}
}

func fixedAttackIntervalMS(config JobStatConfig) int32 {
	if config.AttackIntervalMS <= 0 {
		return DefaultAttackIntervalMS
	}
	return config.AttackIntervalMS
}

func clampInt32(value int32, minValue int32, maxValue int32) int32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func calculateAllocatedStat(stat BaseStat, percent StatPercent) int32 {
	value := float64(stat.Strength)*percent.Strength/100 +
		float64(stat.Intelligence)*percent.Intelligence/100 +
		float64(stat.Agility)*percent.Agility/100 +
		float64(stat.Luck)*percent.Luck/100
	return int32(math.Round(value))
}

func calculateAllocatedFloatStat(stat BaseStat, percent StatPercent) float64 {
	return float64(stat.Strength)*percent.Strength/100 +
		float64(stat.Intelligence)*percent.Intelligence/100 +
		float64(stat.Agility)*percent.Agility/100 +
		float64(stat.Luck)*percent.Luck/100
}

func LoadJobStatConfigs(path string) (JobStatConfigs, error) {
	configs, err := loadJobStatConfigFiles(path)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("job stat config must define jobs")
	}
	return configs, nil
}

func loadJobStatConfigFiles(path string) (JobStatConfigs, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat job stat config: %w", err)
	}
	configs := make(JobStatConfigs)
	if !info.IsDir() {
		if err := loadJobStatConfigFile(path, configs); err != nil {
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
		return loadJobStatConfigFile(filePath, configs)
	}); err != nil {
		return nil, fmt.Errorf("read job stat config directory: %w", err)
	}
	return configs, nil
}

func loadJobStatConfigFile(path string, configs JobStatConfigs) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read job stat config %s: %w", path, err)
	}

	var group JobStatConfigs
	if err := json.Unmarshal(payload, &group); err == nil {
		for id, config := range group {
			configs[id] = config
		}
		return nil
	}

	var config JobStatConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return fmt.Errorf("decode job stat config %s: %w", path, err)
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	configs[id] = config
	return nil
}

func DefaultJobStatConfigs() JobStatConfigs {
	return JobStatConfigs{
		DefaultJobCode: {
			Name: "Beginner",
		},
	}
}
