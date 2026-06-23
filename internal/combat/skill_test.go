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

func TestCalculateSkillTimingUsesCastSpeed(t *testing.T) {
	timing := CalculateSkillTiming(SkillConfig{
		StartupMS:  300,
		ActiveMS:   80,
		RecoveryMS: 420,
	}, SnapshotStat{CastSpeed: 200})

	if timing.IntervalMS != 600 {
		t.Fatalf("expected cast speed to reduce interval to 600ms, got %+v", timing)
	}
	if timing.StartupMS != 225 || timing.ActiveMS != 60 || timing.RecoveryMS != 315 {
		t.Fatalf("expected timing parts to scale with cast speed, got %+v", timing)
	}
}

func TestSkillTimingClampsAtZero(t *testing.T) {
	skill := SkillConfig{StartupMS: 100, ActiveMS: 50, RecoveryMS: 150}
	timing := CalculateSkillTiming(skill, SnapshotStat{CastSpeed: 999})
	if timing != (SkillTiming{}) {
		t.Fatalf("expected cast speed to clamp timing at zero, got %+v", timing)
	}
	if SkillStartup(skill, SnapshotStat{CastSpeed: 999}) != 0 {
		t.Fatal("expected zero startup duration after full cast-speed reduction")
	}
	if SkillCastInterval(skill, SnapshotStat{CastSpeed: 999}) != 0 {
		t.Fatal("expected zero cast interval after full cast-speed reduction")
	}
}
