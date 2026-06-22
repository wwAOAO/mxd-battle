package scene

import "mxd-battle/internal/world"

func resolvePortal(room *room, player Player) (Player, string, bool) {
	player = NormalizePlayerBody(player)
	for _, portal := range room.mapDef.Portals {
		playerBody := world.Rect{
			X:      PlayerLeft(player),
			Y:      PlayerTop(player),
			Width:  player.Width,
			Height: player.Height,
		}
		if world.RectsOverlap(playerBody, portal.Area) {
			player.Room = portal.TargetRoomID
			player.X = portal.Target.X
			player.Y = portal.Target.Y
			player.VX = 0
			player.VY = 0
			player.InputX = 0
			return player, portal.TargetRoomID, true
		}
	}
	return player, "", false
}
