package combat

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type SkillConfigs map[string]SkillConfig

type SkillConfig struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	MPCost     int32             `json:"mpCost"`
	CooldownMS int32             `json:"cooldownMs"`
	StartupMS  int32             `json:"startupMs"`
	ActiveMS   int32             `json:"activeMs"`
	RecoveryMS int32             `json:"recoveryMs"`
	Range      float64           `json:"range"`
	Width      float64           `json:"width"`
	Height     float64           `json:"height"`
	Projectile ProjectileConfig  `json:"projectile"`
	Damage     SkillDamageConfig `json:"damage"`
}

type ProjectileConfig struct {
	Speed       float64 `json:"speed"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	MaxDistance float64 `json:"maxDistance"`
}

type SkillDamageConfig struct {
	Base      int32   `json:"base"`
	MagicRate float64 `json:"magicRate"`
}

type SkillTiming struct {
	StartupMS  int32 `json:"startupMs"`
	ActiveMS   int32 `json:"activeMs"`
	RecoveryMS int32 `json:"recoveryMs"`
	IntervalMS int32 `json:"intervalMs"`
}

func LoadSkillConfigs(path string) (SkillConfigs, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill config: %w", err)
	}

	var configs SkillConfigs
	if err := json.Unmarshal(payload, &configs); err != nil {
		return nil, fmt.Errorf("decode skill config: %w", err)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("skill config must define skills")
	}
	for id, config := range configs {
		if config.ID == "" {
			config.ID = id
			configs[id] = config
		}
	}
	return configs, nil
}

func DefaultSkillConfigs() SkillConfigs {
	return SkillConfigs{
		"magic_missile": {
			ID:         "magic_missile",
			Name:       "Magic Missile",
			MPCost:     12,
			CooldownMS: 900,
			StartupMS:  300,
			ActiveMS:   80,
			RecoveryMS: 420,
			Range:      420,
			Width:      420,
			Height:     72,
			Projectile: ProjectileConfig{
				Speed:       760,
				Width:       48,
				Height:      32,
				MaxDistance: 420,
			},
			Damage: SkillDamageConfig{
				Base:      8,
				MagicRate: 1.2,
			},
		},
	}
}

func CanCastSkill(now time.Time, lastCastAt time.Time, skill SkillConfig) bool {
	if lastCastAt.IsZero() {
		return true
	}
	return !now.Before(lastCastAt.Add(SkillCooldown(skill)))
}

func SkillCooldown(skill SkillConfig) time.Duration {
	if skill.CooldownMS <= 0 {
		return 500 * time.Millisecond
	}
	return time.Duration(skill.CooldownMS) * time.Millisecond
}

func CalculateSkillTiming(skill SkillConfig, stat SnapshotStat) SkillTiming {
	startupMS := maxInt32(0, skill.StartupMS)
	activeMS := maxInt32(0, skill.ActiveMS)
	recoveryMS := maxInt32(0, skill.RecoveryMS)
	baseTotal := startupMS + activeMS + recoveryMS
	if baseTotal <= 0 {
		intervalMS := int32(SkillCooldown(skill) / time.Millisecond)
		intervalMS = maxInt32(0, intervalMS-stat.CastSpeed)
		return SkillTiming{
			RecoveryMS: intervalMS,
			IntervalMS: intervalMS,
		}
	}

	intervalMS := maxInt32(0, baseTotal-stat.CastSpeed)
	if intervalMS == 0 {
		return SkillTiming{}
	}

	scaledStartup := int32(float64(intervalMS)*float64(startupMS)/float64(baseTotal) + 0.5)
	scaledActive := int32(float64(intervalMS)*float64(activeMS)/float64(baseTotal) + 0.5)
	scaledRecovery := intervalMS - scaledStartup - scaledActive
	if scaledRecovery < 0 {
		scaledRecovery = 0
	}

	return SkillTiming{
		StartupMS:  scaledStartup,
		ActiveMS:   scaledActive,
		RecoveryMS: scaledRecovery,
		IntervalMS: intervalMS,
	}
}

func SkillStartup(skill SkillConfig, stat SnapshotStat) time.Duration {
	timing := CalculateSkillTiming(skill, stat)
	if timing.StartupMS <= 0 {
		return 0
	}
	return time.Duration(timing.StartupMS) * time.Millisecond
}

func SkillCastInterval(skill SkillConfig, stat SnapshotStat) time.Duration {
	timing := CalculateSkillTiming(skill, stat)
	if timing.IntervalMS <= 0 {
		return 0
	}
	return time.Duration(timing.IntervalMS) * time.Millisecond
}

func MagicSkillAttack(caster SnapshotStat, target SnapshotStat, skill SkillConfig) AttackOutcome {
	attack := float64(caster.MagicAttackMin+caster.MagicAttackMax) / 2
	damage := int32(attack*skill.Damage.MagicRate) + skill.Damage.Base - target.MagicDefense
	if damage < 1 {
		damage = 1
	}
	return AttackOutcome{Damage: damage, IsHit: true}
}
