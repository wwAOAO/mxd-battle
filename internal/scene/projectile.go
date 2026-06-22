package scene

import (
	"fmt"
	"time"

	"mxd-battle/internal/combat"
	"mxd-battle/internal/world"
)

type Projectile struct {
	ID          string              `json:"id"`
	SkillID     string              `json:"skillId"`
	CasterID    string              `json:"casterId"`
	X           float64             `json:"x"`
	Y           float64             `json:"y"`
	Width       float64             `json:"width"`
	Height      float64             `json:"height"`
	VX          float64             `json:"vx"`
	Distance    float64             `json:"distance"`
	MaxDistance float64             `json:"maxDistance"`
	CreatedAt   time.Time           `json:"createdAt"`
	Skill       combat.SkillConfig  `json:"-"`
	CasterStat  combat.SnapshotStat `json:"-"`
}

func newProjectile(caster Player, skill combat.SkillConfig, now time.Time) Projectile {
	facingX := caster.FacingX
	if facingX == 0 {
		facingX = 1
	}
	width := skill.Projectile.Width
	if width <= 0 {
		width = skill.Width
	}
	if width <= 0 {
		width = 48
	}
	height := skill.Projectile.Height
	if height <= 0 {
		height = skill.Height
	}
	if height <= 0 {
		height = 32
	}
	speed := skill.Projectile.Speed
	if speed <= 0 {
		speed = 760
	}
	maxDistance := skill.Projectile.MaxDistance
	if maxDistance <= 0 {
		maxDistance = skill.Range
	}
	if maxDistance <= 0 {
		maxDistance = 420
	}

	x := PlayerRight(caster)
	if facingX < 0 {
		x = PlayerLeft(caster) - width
	}

	return Projectile{
		ID:          fmt.Sprintf("%s-%d", caster.ID, now.UnixNano()),
		SkillID:     skill.ID,
		CasterID:    caster.ID,
		X:           x,
		Y:           caster.Y - caster.Height + (caster.Height-height)/2,
		Width:       width,
		Height:      height,
		VX:          facingX * speed,
		MaxDistance: maxDistance,
		CreatedAt:   now,
		Skill:       skill,
		CasterStat:  caster.CombatStat,
	}
}

func projectileRect(projectile Projectile) world.Rect {
	return world.Rect{
		X:      projectile.X,
		Y:      projectile.Y,
		Width:  projectile.Width,
		Height: projectile.Height,
	}
}
