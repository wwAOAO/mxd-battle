package scene

import (
	"time"

	"mxd-battle/internal/combat"
	"mxd-battle/internal/world"
)

type movementResult struct {
	Player Player
}

func placePlayerInRoom(player Player, room *room, now time.Time) Player {
	player = NormalizePlayerBody(player)
	player = NormalizePlayerStat(player)
	player.Room = room.id
	player.MapID = room.mapDef.ID
	if player.X == 0 && player.Y == 0 {
		player.X = room.mapDef.Spawn.X
		player.Y = room.mapDef.Spawn.Y
	}
	player.X = world.Clamp(player.X, PlayerHalfWidth(player), room.mapDef.Width-PlayerHalfWidth(player))
	player.Y = world.Clamp(player.Y, 0, room.mapDef.GroundY)
	player = applyGround(room.mapDef, player, player.Y, time.Time{})
	player.UpdatedAt = now
	return player
}

func preparePlayerForRoom(player Player, room *room, jobs combat.JobStatConfigs, now time.Time) Player {
	player = placePlayerInRoom(player, room, now)
	player = NormalizePlayerStatWithJobs(player, jobs)
	player = applySnapshotStat(player, jobs)
	return player
}

func resolvePlayerMovement(room *room, player Player) movementResult {
	previous := room.players[player.ID]
	return resolvePlayerMovementFrom(room, player, previous.X, player.Y, time.Time{})
}

func resolvePlayerMovementFrom(room *room, player Player, previousX float64, previousY float64, now time.Time) movementResult {
	return resolvePlayerMovementFromWithJobs(room, player, previousX, previousY, nil, now)
}

func resolvePlayerMovementFromWithJobs(room *room, player Player, previousX float64, previousY float64, jobs combat.JobStatConfigs, now time.Time) movementResult {
	player = NormalizePlayerBody(player)
	player = normalizePlayerStatForJobs(player, jobs)
	previous := room.players[player.ID]
	previous = NormalizePlayerBody(previous)
	previous = normalizePlayerStatForJobs(previous, jobs)
	previous.X = previousX
	previous.Y = previousY
	player.X = world.Clamp(player.X, PlayerHalfWidth(player), room.mapDef.Width-PlayerHalfWidth(player))
	player.X = resolveWallCollision(room.mapDef, previous, player)
	player.Y = world.Clamp(player.Y, 0, room.mapDef.GroundY)
	player = resolveCeilingCollision(room.mapDef, previous, player)
	player = applyGround(room.mapDef, player, previousY, now)
	player.Room = room.id
	player.MapID = room.mapDef.ID
	if jobs != nil {
		player = applySnapshotStat(player, jobs)
	}

	return movementResult{
		Player: player,
	}
}

func normalizePlayerStatForJobs(player Player, jobs combat.JobStatConfigs) Player {
	if jobs == nil {
		return NormalizePlayerStat(player)
	}
	return NormalizePlayerStatWithJobs(player, jobs)
}

func applySnapshotStat(player Player, jobs combat.JobStatConfigs) Player {
	player.JobCode, player.CombatStat = combat.CalculateSnapshotStat(player.Stat.Final, player.JobCode, jobs)
	return player
}
