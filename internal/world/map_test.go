package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMapsReadsAllTopLevelRoomGroups(t *testing.T) {
	root := t.TempDir()
	mapsDir := filepath.Join(root, "maps")
	if err := os.Mkdir(mapsDir, 0o755); err != nil {
		t.Fatalf("mkdir maps: %v", err)
	}

	writeFile(t, filepath.Join(mapsDir, "map_x.json"), `{"id":"wild_x"}`)
	writeFile(t, filepath.Join(mapsDir, "map_z.json"), `{"id":"wild_z"}`)
	writeFile(t, filepath.Join(root, "world_maps.json"), `{
		"rooms": {
			"X": "maps/map_x.json"
		},
		"rooms2": {
			"Z": "maps/map_z.json"
		}
	}`)

	maps, err := LoadMaps(filepath.Join(root, "world_maps.json"))
	if err != nil {
		t.Fatalf("load maps: %v", err)
	}
	if maps["rooms.X"].ID != "wild_x" {
		t.Fatalf("expected room rooms.X map wild_x, got %+v", maps["rooms.X"])
	}
	if maps["rooms2.Z"].ID != "wild_z" {
		t.Fatalf("expected room rooms2.Z map wild_z, got %+v", maps["rooms2.Z"])
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
