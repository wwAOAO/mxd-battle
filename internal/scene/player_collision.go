package scene

import (
	"math"
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
		if !world.WallHasSolidSides(wall) {
			continue
		}
		next.X = resolveSolidSideCollision(
			world.Rect{X: wall.X, Y: wall.Y, Width: wall.Width, Height: wall.Height},
			previous,
			next,
			nextTop,
			nextBottom,
		)
	}
	for _, terrain := range mapDef.Terrain {
		if !world.TerrainHasSolidSides(terrain) {
			continue
		}
		for _, rect := range terrainSideRects(terrain) {
			next.X = resolveSolidSideCollision(rect, previous, next, nextTop, nextBottom)
		}
	}
	for _, platform := range mapDef.Platforms {
		if !world.PlatformHasSolidSides(platform) {
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
	for _, polygon := range mapDef.Polygons {
		next.X = resolvePolygonSideCollision(polygon, previous, next)
	}

	return next.X
}

func terrainSideRects(terrain world.Terrain) []world.Rect {
	points := world.TerrainPoints(terrain)
	if len(points) < 2 {
		return nil
	}
	rects := make([]world.Rect, 0, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		a := points[i]
		b := points[i+1]
		if math.Abs(a.X-b.X) > wallSkin {
			continue
		}
		top := math.Min(a.Y, b.Y)
		bottom := math.Max(a.Y, b.Y)
		rects = append(rects, world.Rect{
			X:      a.X - wallSkin,
			Y:      top,
			Width:  wallSkin * 2,
			Height: bottom - top,
		})
	}
	return rects
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

func resolvePolygonSideCollision(polygon world.Polygon, previous Player, next Player) float64 {
	bounds, ok := world.PolygonBounds(polygon)
	if !ok || !world.RectOverlapsPolygon(playerRect(next), polygon) {
		return next.X
	}

	previousRight := PlayerRight(previous)
	previousLeft := PlayerLeft(previous)
	nextRight := PlayerRight(next)
	nextLeft := PlayerLeft(next)
	boundsRight := bounds.X + bounds.Width
	movingRight := previousRight <= bounds.X && nextRight >= bounds.X
	movingLeft := previousLeft >= boundsRight && nextLeft <= boundsRight

	if movingRight {
		return bounds.X - PlayerHalfWidth(next) - wallSkin
	}
	if movingLeft {
		return boundsRight + PlayerHalfWidth(next) + wallSkin
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
		if !world.WallHasSolidCeiling(wall) {
			continue
		}
		next = resolveSolidCeilingCollision(
			world.Rect{X: wall.X, Y: wall.Y, Width: wall.Width, Height: wall.Height},
			previousTop,
			nextTop,
			next,
		)
	}
	for _, platform := range mapDef.Platforms {
		if !world.PlatformHasSolidCeiling(platform) {
			continue
		}
		next = resolveSolidCeilingCollision(
			world.Rect{X: platform.X, Y: platform.Y, Width: platform.Width, Height: platform.Height},
			previousTop,
			nextTop,
			next,
		)
	}
	for _, polygon := range mapDef.Polygons {
		next = resolvePolygonCeilingCollision(polygon, previousTop, nextTop, next)
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

func resolvePolygonCeilingCollision(polygon world.Polygon, previousTop float64, nextTop float64, next Player) Player {
	bounds, ok := world.PolygonBounds(polygon)
	if !ok {
		return next
	}

	boundsBottom := bounds.Y + bounds.Height
	onX := PlayerRight(next) > bounds.X && PlayerLeft(next) < bounds.X+bounds.Width
	crossedBottom := previousTop >= boundsBottom && nextTop <= boundsBottom
	if onX && crossedBottom && world.RectOverlapsPolygon(playerRect(next), polygon) {
		next.Y = boundsBottom + next.Height + wallSkin
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
		for _, polygon := range mapDef.Polygons {
			if y, ok := polygonLandingY(polygon, player); ok {
				crossedTop := previousY <= y && player.Y >= y
				if crossedTop && y < landingY {
					landingY = y
				}
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

func polygonLandingY(polygon world.Polygon, player Player) (float64, bool) {
	sampleXs := []float64{PlayerLeft(player), player.X, PlayerRight(player)}
	found := false
	var landingY float64
	for _, x := range sampleXs {
		y, ok := world.PolygonTopYAt(polygon, x)
		if !ok {
			continue
		}
		if !found || y < landingY {
			landingY = y
			found = true
		}
	}
	return landingY, found
}

func playerRect(player Player) world.Rect {
	return world.Rect{
		X:      PlayerLeft(player),
		Y:      PlayerTop(player),
		Width:  player.Width,
		Height: player.Height,
	}
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
