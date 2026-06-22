package combat

import "time"

func CanNormalAttack(now time.Time, lastAttackAt time.Time, stat SnapshotStat) bool {
	if lastAttackAt.IsZero() {
		return true
	}
	return !now.Before(lastAttackAt.Add(AttackInterval(stat)))
}

func IsNormalAttackLocked(now time.Time, lastAttackAt time.Time, stat SnapshotStat) bool {
	if lastAttackAt.IsZero() {
		return false
	}
	return now.Before(lastAttackAt.Add(AttackInterval(stat)))
}
func AttackInterval(stat SnapshotStat) time.Duration {
	intervalMS := stat.AttackIntervalMS
	if intervalMS <= 0 {
		intervalMS = DefaultAttackIntervalMS
	}
	return time.Duration(intervalMS) * time.Millisecond
}
