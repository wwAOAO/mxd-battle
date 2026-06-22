package scene

import (
	"time"

	"mxd-battle/internal/world"
)

const wallSkin = 0.001

func resolveWallCollision(mapDef world.MapConfig, previous Player, next Player) float64 {
	if previous.X == next.X {
		return next.X
	}

	nextTop := PlayerTop(next)
	nextBottom := next.Y
	for _, wall := range mapDef.Walls {
		next.X = resolveSolidSideCollision(
			world.Rect{X: wall.X, Y: wall.Y, Width: wall.Width, Height: wall.Height},
			previous,
			next,
			nextTop,
			nextBottom,
		)
	}
	for _, platform := range mapDef.Platforms {
		if !platform.SolidSides {
			continue
		}
		next.X = resolveSolidSideCollision(
			world.Rect{X: platform.X, Y: platform.Y, Width: platform.Width, Height: platform.Height},
			previous,
			next,
			nextTop,
			nextBottom,
		)
	}

	return next.X
}

func resolveSolidSideCollision(rect world.Rect, previous Player, next Player, nextTop float64, nextBottom float64) float64 {
	if nextBottom <= rect.Y || !world.OverlapsVertical(nextTop, nextBottom, rect) {
		return next.X
	}

	rectLeft := rect.X
	rectRight := rect.X + rect.Width
	previousRight := PlayerRight(previous)
	previousLeft := PlayerLeft(previous)
	nextRight := PlayerRight(next)
	nextLeft := PlayerLeft(next)
	movingRight := previousRight <= rectLeft && nextRight >= rectLeft
	movingLeft := previousLeft >= rectRight && nextLeft <= rectRight

	if movingRight {
		return rectLeft - PlayerHalfWidth(next) - wallSkin
	}
	if movingLeft {
		return rectRight + PlayerHalfWidth(next) + wallSkin
	}
	return next.X
}

func resolveCeilingCollision(mapDef world.MapConfig, previous Player, next Player) Player {
	if next.VY >= 0 {
		return next
	}

	previousTop := PlayerTop(previous)
	nextTop := PlayerTop(next)
	for _, wall := range mapDef.Walls {
		next = resolveSolidCeilingCollision(
			world.Rect{X: wall.X, Y: wall.Y, Width: wall.Width, Height: wall.Height},
			previousTop,
			nextTop,
			next,
		)
	}
	for _, platform := range mapDef.Platforms {
		if !platform.SolidCeiling {
			continue
		}
		next = resolveSolidCeilingCollision(
			world.Rect{X: platform.X, Y: platform.Y, Width: platform.Width, Height: platform.Height},
			previousTop,
			nextTop,
			next,
		)
	}
	return next
}

func resolveSolidCeilingCollision(rect world.Rect, previousTop float64, nextTop float64, next Player) Player {
	rectBottom := rect.Y + rect.Height
	onX := PlayerRight(next) > rect.X && PlayerLeft(next) < rect.X+rect.Width
	crossedBottom := previousTop >= rectBottom && nextTop <= rectBottom
	if onX && crossedBottom {
		next.Y = rectBottom + next.Height + wallSkin
		next.VY = 0
	}
	return next
}

func applyGround(mapDef world.MapConfig, player Player, previousY float64, now time.Time) Player {
	landingY := terrainLandingY(mapDef, player.X)

	if player.VY >= 0 {
		for _, platform := range mapDef.Platforms {
			if shouldIgnorePlatform(player, platform, now) {
				continue
			}
			onX := PlayerRight(player) >= platform.X && PlayerLeft(player) <= platform.X+platform.Width
			crossedTop := previousY <= platform.Y && player.Y >= platform.Y
			if onX && crossedTop && platform.Y < landingY {
				landingY = platform.Y
			}
		}
		for _, wall := range mapDef.Walls {
			onX := PlayerRight(player) >= wall.X && PlayerLeft(player) <= wall.X+wall.Width
			crossedTop := previousY <= wall.Y && player.Y >= wall.Y
			if onX && crossedTop && wall.Y < landingY {
				landingY = wall.Y
			}
		}
	}

	if player.Y >= landingY {
		player.Y = landingY
		player.VY = 0
		player.OnGround = true
		return player
	}
	player.OnGround = false
	return player
}

func terrainLandingY(mapDef world.MapConfig, x float64) float64 {
	landingY := mapDef.GroundY
	for _, terrain := range mapDef.Terrain {
		y, ok := world.TerrainYAt(terrain, x)
		if ok && y < landingY {
			landingY = y
		}
	}
	return landingY
}

func shouldIgnorePlatform(player Player, platform world.Platform, now time.Time) bool {
	if player.DropUntil.IsZero() || now.IsZero() || !now.Before(player.DropUntil) {
		return false
	}
	return previousFootWasOnPlatform(player, platform)
}

func isOnDropThroughPlatform(mapDef world.MapConfig, player Player) bool {
	for _, platform := range mapDef.Platforms {
		if previousFootWasOnPlatform(player, platform) {
			return true
		}
	}
	return false
}

func previousFootWasOnPlatform(player Player, platform world.Platform) bool {
	onX := PlayerRight(player) >= platform.X && PlayerLeft(player) <= platform.X+platform.Width
	return onX && player.Y == platform.Y
}
