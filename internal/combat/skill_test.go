package combat

import (
	"testing"
	"time"
)

func TestMagicSkillAttackUsesMagicAttackAndSkillConfig(t *testing.T) {
	outcome := MagicSkillAttack(
		SnapshotStat{MagicAttackMin: 20, MagicAttackMax: 30},
		SnapshotStat{MagicDefense: 5},
		SkillConfig{Damage: SkillDamageConfig{Base: 8, MagicRate: 1.2}},
	)

	if !outcome.IsHit || outcome.Damage != 33 {
		t.Fatalf("expected configured magic skill damage, got %+v", outcome)
	}
}

func TestCanCastSkillUsesCooldown(t *testing.T) {
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	skill := SkillConfig{CooldownMS: 900}

	if !CanCastSkill(now, time.Time{}, skill) {
		t.Fatal("expected first cast to be allowed")
	}
	if CanCastSkill(now.Add(899*time.Millisecond), now, skill) {
		t.Fatal("expected cast inside cooldown to be blocked")
	}
	if !CanCastSkill(now.Add(900*time.Millisecond), now, skill) {
		t.Fatal("expected cast at cooldown boundary to be allowed")
	}
}
