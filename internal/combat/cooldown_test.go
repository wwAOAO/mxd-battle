package combat

import (
	"testing"
	"time"
)

func TestCanNormalAttackUsesAttackInterval(t *testing.T) {
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	stat := SnapshotStat{AttackIntervalMS: 600}

	if !CanNormalAttack(now, time.Time{}, stat) {
		t.Fatal("expected first normal attack to be allowed")
	}
	if CanNormalAttack(now.Add(599*time.Millisecond), now, stat) {
		t.Fatal("expected normal attack inside interval to be blocked")
	}
	if !CanNormalAttack(now.Add(600*time.Millisecond), now, stat) {
		t.Fatal("expected normal attack at interval boundary to be allowed")
	}
}

func TestCalculateSnapshotStatCalculatesAttackIntervalFromStatReduction(t *testing.T) {
	_, stat := CalculateSnapshotStat(BaseStat{Agility: 50, Luck: 10}, "beginner", JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			AttackInterval: AttackIntervalConfig{
				BaseMS:     700,
				MinMS:      250,
				MaxMS:      1500,
				StartupMS:  180,
				ActiveMS:   120,
				RecoveryMS: 400,
				Reduction: StatReduction{
					Agility: 2,
					Luck:    0.5,
				},
			},
		},
	})

	if stat.AttackIntervalMS != 595 {
		t.Fatalf("expected attack interval to be reduced by agility and luck, got %+v", stat)
	}
	if stat.AttackStartupMS+stat.AttackActiveMS+stat.AttackRecoveryMS != stat.AttackIntervalMS {
		t.Fatalf("expected attack timing parts to equal interval, got %+v", stat)
	}
}

func TestCalculateSnapshotStatClampsAttackInterval(t *testing.T) {
	configs := JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			AttackInterval: AttackIntervalConfig{
				BaseMS: 700,
				MinMS:  250,
				MaxMS:  900,
				Reduction: StatReduction{
					Agility: 20,
				},
			},
		},
	}

	_, minStat := CalculateSnapshotStat(BaseStat{Agility: 999}, "beginner", configs)
	if minStat.AttackIntervalMS != 250 {
		t.Fatalf("expected attack interval to clamp to min, got %+v", minStat)
	}

	_, maxStat := CalculateSnapshotStat(BaseStat{}, "beginner", JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			AttackInterval: AttackIntervalConfig{
				BaseMS: 2000,
				MinMS:  250,
				MaxMS:  900,
			},
		},
	})
	if maxStat.AttackIntervalMS != 900 {
		t.Fatalf("expected attack interval to clamp to max, got %+v", maxStat)
	}
}

func TestCalculateSnapshotStatKeepsFixedAttackIntervalCompatibility(t *testing.T) {
	_, stat := CalculateSnapshotStat(BaseStat{Agility: 999}, "beginner", JobStatConfigs{
		"beginner": {
			Name:             "Beginner",
			AttackIntervalMS: 600,
		},
	})

	if stat.AttackIntervalMS != 600 {
		t.Fatalf("expected fixed attack interval to be used, got %+v", stat)
	}
	if stat.AttackStartupMS+stat.AttackActiveMS+stat.AttackRecoveryMS != stat.AttackIntervalMS {
		t.Fatalf("expected fixed attack interval to be split into timing parts, got %+v", stat)
	}
}

func TestCalculateSnapshotStatCalculatesRecoveryStats(t *testing.T) {
	_, stat := CalculateSnapshotStat(BaseStat{Strength: 20, Intelligence: 30, Luck: 10}, "beginner", JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			Allocation: CombatStatAllocation{
				HPRecovery: StatPercent{Strength: 10, Luck: 5},
				MPRecovery: StatPercent{Intelligence: 10, Luck: 5},
			},
		},
	})

	if stat.HPRecovery != 2.5 || stat.MPRecovery != 3.5 {
		t.Fatalf("expected recovery stats to be calculated, got %+v", stat)
	}
}
