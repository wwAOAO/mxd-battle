package scene

import (
	"math"
	"time"

	"mxd-battle/internal/world"
)

const wallSkin = 0.001
const maxWalkableTerrainStep = 48.0
const maxWalkableTerrainSlope = 0.8

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
		for _, side := range terrainSideRects(terrain) {
			next.X = resolveTerrainSideCollision(side, previous, next, nextTop, nextBottom)
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

type terrainSideRect struct {
	rect     world.Rect
	highSide int
}

func terrainSideRects(terrain world.Terrain) []terrainSideRect {
	points := world.TerrainPoints(terrain)
	if len(points) < 2 {
		return nil
	}
	rects := make([]terrainSideRect, 0, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		a := points[i]
		b := points[i+1]
		top := math.Min(a.Y, b.Y)
		bottom := math.Max(a.Y, b.Y)
		if math.Abs(a.X-b.X) > wallSkin {
			if !isSteepTerrainStep(a, b) {
				continue
			}
			rects = append(rects, terrainSideRect{
				rect: world.Rect{
					X:      lowerTerrainEntryX(a, b) - wallSkin,
					Y:      top,
					Width:  wallSkin * 2,
					Height: bottom - top,
				},
				highSide: terrainHighSide(points, i),
			})
			continue
		}
		rects = append(rects, terrainSideRect{
			rect: world.Rect{
				X:      a.X - wallSkin,
				Y:      top,
				Width:  wallSkin * 2,
				Height: bottom - top,
			},
			highSide: terrainHighSide(points, i),
		})
	}
	return rects
}

func terrainHighSide(points []world.Point, index int) int {
	a := points[index]
	b := points[index+1]
	if a.Y < b.Y {
		return terrainEndpointSide(points, index, -1)
	}
	if b.Y < a.Y {
		return terrainEndpointSide(points, index+1, 1)
	}
	return 0
}

func terrainEndpointSide(points []world.Point, index int, fallback int) int {
	point := points[index]
	if index > 0 && points[index-1].Y == point.Y {
		return sideOf(points[index-1].X, point.X, fallback)
	}
	if index+1 < len(points) && points[index+1].Y == point.Y {
		return sideOf(points[index+1].X, point.X, fallback)
	}
	return fallback
}

func sideOf(x float64, origin float64, fallback int) int {
	if x < origin {
		return -1
	}
	if x > origin {
		return 1
	}
	return fallback
}
func isSteepTerrainStep(a world.Point, b world.Point) bool {
	rise := math.Abs(a.Y - b.Y)
	run := math.Abs(a.X - b.X)
	if rise <= maxWalkableTerrainStep || run <= wallSkin {
		return false
	}
	return rise/run > maxWalkableTerrainSlope
}

func lowerTerrainEntryX(a world.Point, b world.Point) float64 {
	if a.Y > b.Y {
		return a.X
	}
	return b.X
}

func resolveTerrainSideCollision(side terrainSideRect, previous Player, next Player, nextTop float64, nextBottom float64) float64 {
	rect := side.rect
	if nextBottom <= rect.Y || !world.OverlapsVertical(nextTop, nextBottom, rect) {
		return next.X
	}
	rectLeft := rect.X
	rectRight := rect.X + rect.Width
	nextRight := PlayerRight(next)
	nextLeft := PlayerLeft(next)
	movingRight := previous.X <= rectLeft && nextRight >= rectLeft
	movingLeft := previous.X >= rectRight && nextLeft <= rectRight

	if movingRight {
		if side.highSide < 0 || side.highSide > 0 && terrainSideStepOverY(previous, next) <= rect.Y+maxWalkableTerrainStep {
			return next.X
		}
		return rectLeft - PlayerHalfWidth(next) - wallSkin
	}
	if movingLeft {
		if side.highSide > 0 || side.highSide < 0 && terrainSideStepOverY(previous, next) <= rect.Y+maxWalkableTerrainStep {
			return next.X
		}
		return rectRight + PlayerHalfWidth(next) + wallSkin
	}
	return next.X
}
func terrainSideStepOverY(previous Player, next Player) float64 {
	return math.Min(previous.Y, next.Y)
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
	landingY := terrainLandingY(mapDef, player)

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

func terrainLandingY(mapDef world.MapConfig, player Player) float64 {
	landingY := mapDef.GroundY
	centerY := terrainYAt(mapDef, player.X, landingY)
	if centerY < landingY {
		landingY = centerY
	}

	// While moving upward, only the player's center can claim terrain support.
	// This prevents the jump arc from visually "sliding" onto higher ground
	// just because a side edge brushes the platform lip mid-air.
	if player.VY < 0 {
		return landingY
	}

	for _, x := range []float64{PlayerLeft(player), PlayerRight(player)} {
		if terrainSideBetween(mapDef, player.X, x, player.Y) {
			continue
		}
		y := terrainEdgeYAt(mapDef, x, player.Y, mapDef.GroundY)
		if centerY-y <= maxWalkableTerrainStep {
			continue
		}
		if y < landingY {
			landingY = y
		}
	}
	return landingY
}

func terrainSideBetween(mapDef world.MapConfig, fromX float64, toX float64, footY float64) bool {
	minX := math.Min(fromX, toX)
	maxX := math.Max(fromX, toX)
	for _, terrain := range mapDef.Terrain {
		if !world.TerrainHasSolidSides(terrain) {
			continue
		}
		for _, side := range terrainSideRects(terrain) {
			rect := side.rect
			sideX := rect.X + rect.Width/2
			if sideX <= minX || sideX >= maxX {
				continue
			}
			if footY > rect.Y+maxWalkableTerrainStep && footY <= rect.Y+rect.Height+maxWalkableTerrainStep {
				return true
			}
		}
	}
	return false
}
func terrainEdgeYAt(mapDef world.MapConfig, x float64, footY float64, fallback float64) float64 {
	landingY := fallback
	for _, terrain := range mapDef.Terrain {
		y, ok := terrainSurfaceYAtForEdge(terrain, x, footY)
		if ok && y < landingY {
			landingY = y
		}
	}
	return landingY
}

func terrainSurfaceYAtForEdge(terrain world.Terrain, x float64, footY float64) (float64, bool) {
	points := world.TerrainPoints(terrain)
	for i := 0; i < len(points)-1; i++ {
		a := points[i]
		b := points[i+1]
		if math.Abs(a.X-b.X) <= wallSkin || isSteepTerrainStep(a, b) {
			continue
		}
		minX := math.Min(a.X, b.X)
		maxX := math.Max(a.X, b.X)
		if x < minX || x > maxX || terrainSurfaceEndpointTouchesWall(points, i, x) {
			continue
		}
		t := (x - a.X) / (b.X - a.X)
		y := a.Y + (b.Y-a.Y)*t
		if y < footY-maxWalkableTerrainStep && terrainSurfaceEndpointNearSolidSide(points, i, x) {
			continue
		}
		return y, true
	}
	return 0, false
}

func terrainSurfaceEndpointNearSolidSide(points []world.Point, segmentIndex int, x float64) bool {
	return math.Abs(x-points[segmentIndex].X) <= PlayerHalfWidth(Player{Width: DefaultPlayerWidth}) && terrainPointTouchesSolidSide(points, segmentIndex) ||
		math.Abs(x-points[segmentIndex+1].X) <= PlayerHalfWidth(Player{Width: DefaultPlayerWidth}) && terrainPointTouchesSolidSide(points, segmentIndex+1)
}

func terrainPointTouchesSolidSide(points []world.Point, pointIndex int) bool {
	if pointIndex > 0 && isTerrainSolidSideSegment(points[pointIndex-1], points[pointIndex]) {
		return true
	}
	return pointIndex+1 < len(points) && isTerrainSolidSideSegment(points[pointIndex], points[pointIndex+1])
}

func isTerrainSolidSideSegment(a world.Point, b world.Point) bool {
	return math.Abs(a.X-b.X) <= wallSkin || isSteepTerrainStep(a, b)
}
func terrainYAt(mapDef world.MapConfig, x float64, fallback float64) float64 {
	landingY := fallback
	for _, terrain := range mapDef.Terrain {
		y, ok := terrainSurfaceYAt(terrain, x)
		if ok && y < landingY {
			landingY = y
		}
	}
	return landingY
}

func terrainSurfaceYAt(terrain world.Terrain, x float64) (float64, bool) {
	points := world.TerrainPoints(terrain)
	for i := 0; i < len(points)-1; i++ {
		a := points[i]
		b := points[i+1]
		if math.Abs(a.X-b.X) <= wallSkin || isSteepTerrainStep(a, b) {
			continue
		}
		minX := math.Min(a.X, b.X)
		maxX := math.Max(a.X, b.X)
		if x < minX || x > maxX || terrainSurfaceEndpointTouchesWall(points, i, x) {
			continue
		}
		t := (x - a.X) / (b.X - a.X)
		return a.Y + (b.Y-a.Y)*t, true
	}
	return 0, false
}

func terrainSurfaceEndpointTouchesWall(points []world.Point, segmentIndex int, x float64) bool {
	if math.Abs(x-points[segmentIndex].X) <= wallSkin {
		return terrainPointTouchesVerticalSegment(points, segmentIndex)
	}
	if math.Abs(x-points[segmentIndex+1].X) <= wallSkin {
		return terrainPointTouchesVerticalSegment(points, segmentIndex+1)
	}
	return false
}

func terrainPointTouchesVerticalSegment(points []world.Point, pointIndex int) bool {
	point := points[pointIndex]
	if pointIndex > 0 && math.Abs(points[pointIndex-1].X-point.X) <= wallSkin {
		return true
	}
	return pointIndex+1 < len(points) && math.Abs(points[pointIndex+1].X-point.X) <= wallSkin
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
