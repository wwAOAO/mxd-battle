package scene

import "mxd-battle/internal/world"

const (
	DefaultLadderClimbSpeed = 220.0
	ladderTopAttachGap      = 6.0
)

func tryAttachLadder(mapDef world.MapConfig, player Player) Player {
	ladder, ok := overlappingLadder(mapDef, player)
	if !ok {
		return detachLadder(player)
	}
	player.OnLadder = true
	player.LadderID = ladder.ID
	player.OnGround = false
	player.VX = 0
	player.VY = 0
	player.X = world.Clamp(player.X, ladder.X, ladder.X+ladder.Width)
	return player
}

func syncLadderState(mapDef world.MapConfig, player Player) Player {
	if !player.OnLadder {
		return player
	}
	ladder, ok := ladderByID(mapDef, player.LadderID)
	if !ok || !playerOverlapsLadder(player, ladder) {
		return detachLadder(player)
	}
	player.X = world.Clamp(player.X, ladder.X, ladder.X+ladder.Width)
	return player
}

func detachLadder(player Player) Player {
	player.OnLadder = false
	player.LadderID = ""
	return player
}

func overlappingLadder(mapDef world.MapConfig, player Player) (world.Ladder, bool) {
	playerBounds := playerRect(player)
	for _, ladder := range mapDef.Ladders {
		ladderRect := world.Rect{X: ladder.X, Y: ladder.Y, Width: ladder.Width, Height: ladder.Height}
		if world.RectsOverlap(playerBounds, ladderRect) || canAttachFromLadderTop(player, ladder) {
			return ladder, true
		}
	}
	return world.Ladder{}, false
}

func canAttachFromLadderTop(player Player, ladder world.Ladder) bool {
	centerWithin := player.X >= ladder.X && player.X <= ladder.X+ladder.Width
	feetNearTop := player.Y >= ladder.Y-ladderTopAttachGap && player.Y <= ladder.Y+ladderTopAttachGap
	return centerWithin && feetNearTop
}

func playerOverlapsLadder(player Player, ladder world.Ladder) bool {
	return world.RectsOverlap(playerRect(player), world.Rect{X: ladder.X, Y: ladder.Y, Width: ladder.Width, Height: ladder.Height})
}

func ladderByID(mapDef world.MapConfig, ladderID string) (world.Ladder, bool) {
	for _, ladder := range mapDef.Ladders {
		if ladder.ID == ladderID {
			return ladder, true
		}
	}
	return world.Ladder{}, false
}

func ladderClimbSpeed(ladder world.Ladder) float64 {
	if ladder.ClimbSpeed > 0 {
		return ladder.ClimbSpeed
	}
	return DefaultLadderClimbSpeed
}
