package world

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type MapConfig struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Width        float64    `json:"width"`
	Height       float64    `json:"height"`
	GroundY      float64    `json:"groundY"`
	Gravity      float64    `json:"gravity"`
	JumpVelocity float64    `json:"jumpVelocity"`
	MoveSpeed    float64    `json:"moveSpeed"`
	Spawn        Point      `json:"spawn"`
	Terrain      []Terrain  `json:"terrain,omitempty"`
	Polygons     []Polygon  `json:"polygons,omitempty"`
	Platforms    []Platform `json:"platforms"`
	Walls        []Wall     `json:"walls"`
	Portals      []Portal   `json:"portals"`
}

type MapsConfig map[string]map[string]string

func LoadMaps(path string) (map[string]MapConfig, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read world maps file: %w", err)
	}

	var config MapsConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return nil, fmt.Errorf("decode world maps file: %w", err)
	}

	baseDir := filepath.Dir(path)
	maps := make(map[string]MapConfig)
	for groupName, rooms := range config {
		if len(rooms) == 0 {
			continue
		}
		for roomID, mapPath := range rooms {
			fullRoomID := groupName + "." + roomID
			if _, exists := maps[fullRoomID]; exists {
				return nil, fmt.Errorf("world maps config duplicate room %q", fullRoomID)
			}
			if mapPath == "" {
				return nil, fmt.Errorf("world maps config group %q room %q missing map file", groupName, roomID)
			}
			if !filepath.IsAbs(mapPath) {
				mapPath = filepath.Join(baseDir, mapPath)
			}
			mapDef, err := LoadMap(mapPath)
			if err != nil {
				return nil, fmt.Errorf("load group %q room %q map: %w", groupName, roomID, err)
			}
			maps[fullRoomID] = mapDef
		}
	}
	if err := ValidateRoomMaps(maps); err != nil {
		return nil, err
	}
	return cloneRoomMaps(maps), nil
}

func LoadMap(path string) (MapConfig, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return MapConfig{}, fmt.Errorf("read map file: %w", err)
	}

	var mapDef MapConfig
	if err := json.Unmarshal(payload, &mapDef); err != nil {
		return MapConfig{}, fmt.Errorf("decode map file: %w", err)
	}
	return mapDef, nil
}

func ValidateRoomMaps(maps map[string]MapConfig) error {
	if len(maps) == 0 {
		return fmt.Errorf("world maps config must define rooms")
	}
	for roomID, mapDef := range maps {
		if roomID == "" {
			return fmt.Errorf("world maps config contains empty room id")
		}
		if !strings.Contains(roomID, ".") {
			return fmt.Errorf("world maps config room %q must use group.room format", roomID)
		}
		if mapDef.ID == "" {
			return fmt.Errorf("world maps config room %q missing map id", roomID)
		}
	}
	return nil
}

func cloneTerrain(source []Terrain) []Terrain {
	cloned := slices.Clone(source)
	for i := range cloned {
		cloned[i].Points = slices.Clone(cloned[i].Points)
	}
	return cloned
}

func clonePolygons(source []Polygon) []Polygon {
	cloned := slices.Clone(source)
	for i := range cloned {
		cloned[i].Points = slices.Clone(cloned[i].Points)
	}
	return cloned
}

func cloneRoomMaps(source map[string]MapConfig) map[string]MapConfig {
	cloned := make(map[string]MapConfig, len(source))
	for roomID, mapDef := range source {
		mapDef.Terrain = cloneTerrain(mapDef.Terrain)
		mapDef.Polygons = clonePolygons(mapDef.Polygons)
		mapDef.Platforms = slices.Clone(mapDef.Platforms)
		mapDef.Walls = slices.Clone(mapDef.Walls)
		mapDef.Portals = slices.Clone(mapDef.Portals)
		cloned[roomID] = mapDef
	}
	return cloned
}
