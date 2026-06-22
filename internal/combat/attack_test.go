package combat

import "testing"

func TestNormalAttackUsesPhysicalAttackAgainstDefense(t *testing.T) {
	outcome := NormalAttack(
		SnapshotStat{PhysicalAttackMin: 10, PhysicalAttackMax: 20},
		SnapshotStat{PhysicalDefense: 4},
	)

	if !outcome.IsHit || outcome.Damage != 11 {
		t.Fatalf("expected normal attack to deal averaged damage minus defense, got %+v", outcome)
	}
}

func TestNormalAttackDealsAtLeastOneDamage(t *testing.T) {
	outcome := NormalAttack(
		SnapshotStat{PhysicalAttackMin: 1, PhysicalAttackMax: 1},
		SnapshotStat{PhysicalDefense: 999},
	)

	if outcome.Damage != 1 {
		t.Fatalf("expected minimum damage to be 1, got %+v", outcome)
	}
}
