package scene

import (
	"maps"

	"mxd-battle/internal/world"
)

type room struct {
	id            string
	mapDef        world.MapConfig
	players       map[string]Player
	projectiles   map[string]Projectile
	pendingSkills map[string]PendingSkill
	peers         map[string]Peer
}

func newRoom(id string, mapDef world.MapConfig) *room {
	return &room{
		id:            id,
		mapDef:        mapDef,
		players:       make(map[string]Player),
		projectiles:   make(map[string]Projectile),
		pendingSkills: make(map[string]PendingSkill),
		peers:         make(map[string]Peer),
	}
}

func roomSnapshot(room *room) RoomState {
	return RoomState{
		ID:          room.id,
		Map:         room.mapDef,
		Players:     maps.Clone(room.players),
		Projectiles: maps.Clone(room.projectiles),
	}
}

func roomPeersExcept(room *room, excludedPlayerID string) map[string]Peer {
	peers := make(map[string]Peer, len(room.peers))
	for playerID, peer := range room.peers {
		if playerID != excludedPlayerID {
			peers[playerID] = peer
		}
	}
	return peers
}

func roomPeers(room *room) map[string]Peer {
	peers := make(map[string]Peer, len(room.peers))
	for playerID, peer := range room.peers {
		peers[playerID] = peer
	}
	return peers
}
