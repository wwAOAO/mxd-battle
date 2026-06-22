package combat

import "math"

type AttackOutcome struct {
	Damage     int32 `json:"damage"`
	IsHit      bool  `json:"isHit"`
	IsCritical bool  `json:"isCritical"`
}

func NormalAttack(attacker SnapshotStat, target SnapshotStat) AttackOutcome {
	attack := (attacker.PhysicalAttackMin + attacker.PhysicalAttackMax) / 2
	defense := target.PhysicalDefense
	damage := attack - defense
	if damage < 1 {
		damage = 1
	}

	return AttackOutcome{
		Damage: int32(math.Max(1, float64(damage))),
		IsHit:  true,
	}
}
