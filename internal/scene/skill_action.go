package scene

import (
	"time"

	"mxd-battle/internal/combat"
)

const (
	actionKindNormalAttack = "normal_attack"
	actionKindSkill        = "skill"
)

type PendingSkill struct {
	ID        string
	CasterID  string
	SkillID   string
	Skill     combat.SkillConfig
	ReadyAt   time.Time
	CreatedAt time.Time
}

func isActionLocked(player Player, now time.Time) bool {
	return !player.ActionLockedUntil.IsZero() && now.Before(player.ActionLockedUntil)
}

func clearExpiredAction(player Player, now time.Time) Player {
	if !isActionLocked(player, now) {
		player.ActionKind = ""
		player.ActionLockedUntil = time.Time{}
	}
	return player
}

func pendingSkillID(casterID string, skillID string, now time.Time) string {
	return casterID + "-" + skillID + "-" + now.Format("20060102150405.000000000")
}
