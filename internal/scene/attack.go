package scene

import (
	"mxd-battle/internal/combat"
	"mxd-battle/internal/world"
)

const (
	normalAttackRange  = 96
	normalAttackHeight = 72
)

func normalAttackArea(player Player) world.Rect {
	facingX := player.FacingX
	if facingX == 0 {
		facingX = 1
	}

	y := player.Y - player.Height + (player.Height-normalAttackHeight)/2
	if facingX < 0 {
		return world.Rect{
			X:      PlayerLeft(player) - normalAttackRange,
			Y:      y,
			Width:  normalAttackRange,
			Height: normalAttackHeight,
		}
	}
	return world.Rect{
		X:      PlayerRight(player),
		Y:      y,
		Width:  normalAttackRange,
		Height: normalAttackHeight,
	}
}

func playerIntersectsRect(player Player, rect world.Rect) bool {
	playerLeft := PlayerLeft(player)
	playerRight := PlayerRight(player)
	playerTop := PlayerTop(player)
	playerBottom := player.Y
	rectRight := rect.X + rect.Width
	rectBottom := rect.Y + rect.Height

	return playerRight > rect.X &&
		playerLeft < rectRight &&
		playerBottom > rect.Y &&
		playerTop < rectBottom
}

func skillArea(player Player, skill combat.SkillConfig) world.Rect {
	facingX := player.FacingX
	if facingX == 0 {
		facingX = 1
	}
	width := skill.Width
	if width <= 0 {
		width = skill.Range
	}
	if width <= 0 {
		width = 240
	}
	height := skill.Height
	if height <= 0 {
		height = 72
	}

	y := player.Y - player.Height + (player.Height-height)/2
	if facingX < 0 {
		return world.Rect{
			X:      PlayerLeft(player) - width,
			Y:      y,
			Width:  width,
			Height: height,
		}
	}
	return world.Rect{
		X:      PlayerRight(player),
		Y:      y,
		Width:  width,
		Height: height,
	}
}
