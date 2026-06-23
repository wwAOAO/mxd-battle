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

func preparePlayerForRoom(player Player, room *room, jobs combat.JobStatConfigs, equipmentConfigs combat.EquipmentConfigs, now time.Time) Player {
	player = placePlayerInRoom(player, room, now)
	player.EquipmentIDs = FilterEquipmentsByRequirement(player, player.EquipmentIDs, equipmentConfigs)
	player = ApplyEquipmentStats(player, equipmentConfigs)
	player = NormalizePlayerStatWithJobs(player, jobs)
	player = applySnapshotStat(player, jobs, equipmentConfigs)
	return player
}

func resolvePlayerMovement(room *room, player Player) movementResult {
	previous := room.players[player.ID]
	return resolvePlayerMovementFrom(room, player, previous.X, player.Y, time.Time{})
}

func resolvePlayerMovementFrom(room *room, player Player, previousX float64, previousY float64, now time.Time) movementResult {
	return resolvePlayerMovementFromWithJobs(room, player, previousX, previousY, nil, nil, now)
}

func resolvePlayerMovementFromWithJobs(room *room, player Player, previousX float64, previousY float64, jobs combat.JobStatConfigs, equipmentConfigs combat.EquipmentConfigs, now time.Time) movementResult {
	player = NormalizePlayerBody(player)
	player = normalizePlayerStatForJobs(player, jobs, equipmentConfigs)
	previous := room.players[player.ID]
	previous = NormalizePlayerBody(previous)
	previous = normalizePlayerStatForJobs(previous, jobs, equipmentConfigs)
	previous.X = previousX
	previous.Y = previousY
	player.X = world.Clamp(player.X, PlayerHalfWidth(player), room.mapDef.Width-PlayerHalfWidth(player))
	player.X = resolveWallCollision(room.mapDef, previous, player)
	player.Y = world.Clamp(player.Y, 0, room.mapDef.GroundY)
	player = syncLadderState(room.mapDef, player)
	if !player.OnLadder {
		player = resolveCeilingCollision(room.mapDef, previous, player)
		player = applyGround(room.mapDef, player, previousY, now)
	} else {
		player.OnGround = false
	}
	player.Room = room.id
	player.MapID = room.mapDef.ID
	if jobs != nil {
		player = applySnapshotStat(player, jobs, equipmentConfigs)
	}

	return movementResult{
		Player: player,
	}
}

func normalizePlayerStatForJobs(player Player, jobs combat.JobStatConfigs, equipmentConfigs combat.EquipmentConfigs) Player {
	player.EquipmentIDs = FilterEquipmentsByRequirement(player, player.EquipmentIDs, equipmentConfigs)
	player = ApplyEquipmentStats(player, equipmentConfigs)
	if jobs == nil {
		return NormalizePlayerStat(player)
	}
	return NormalizePlayerStatWithJobs(player, jobs)
}

func applySnapshotStat(player Player, jobs combat.JobStatConfigs, equipmentConfigs combat.EquipmentConfigs) Player {
	player.JobCode, player.CombatStat = combat.CalculateSnapshotStat(player.Stat.Final, player.JobCode, jobs)
	player.CombatStat = combat.ApplyEquipmentCombatStat(player.CombatStat, combat.AggregateEquipmentCombatStat(player.EquipmentIDs, equipmentConfigs))
	return player
}
