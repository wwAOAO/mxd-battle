package scene

import (
	"fmt"
	"time"

	"mxd-battle/internal/world"
)

const (
	defaultLootPickupRange = 96.0
	defaultLootLifetime    = 45 * time.Second
)

type LootDrop struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	SourceType  string    `json:"sourceType"`
	EquipmentID string    `json:"equipmentId,omitempty"`
	Name        string    `json:"name"`
	X           float64   `json:"x"`
	Y           float64   `json:"y"`
	Width       float64   `json:"width"`
	Height      float64   `json:"height"`
	OwnerID     string    `json:"ownerId,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

func newLootDrop(monster Monster, attackerID string, equipmentID string, name string, now time.Time) LootDrop {
	return LootDrop{
		ID:          fmt.Sprintf("loot_%s_%d", monster.ID, now.UnixNano()),
		SourceID:    monster.ID,
		SourceType:  "monster",
		EquipmentID: equipmentID,
		Name:        name,
		X:           monster.X,
		Y:           monster.Y,
		Width:       34,
		Height:      24,
		OwnerID:     attackerID,
		ExpiresAt:   now.Add(defaultLootLifetime),
		CreatedAt:   now,
	}
}

func lootRect(drop LootDrop) world.Rect {
	return world.Rect{
		X:      drop.X - drop.Width/2,
		Y:      drop.Y - drop.Height,
		Width:  drop.Width,
		Height: drop.Height,
	}
}

func canPickupLoot(player Player, drop LootDrop) bool {
	if drop.OwnerID != "" && drop.OwnerID != player.ID {
		return false
	}
	return absFloat64(player.X-drop.X) <= defaultLootPickupRange &&
		absFloat64(player.Y-drop.Y) <= player.Height+drop.Height+24
}
