package combat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalAttackUsesPhysicalAttackAgainstDefense(t *testing.T) {
	outcome := NormalAttack(
		SnapshotStat{PhysicalAttackMin: 10, PhysicalAttackMax: 20},
		SnapshotStat{PhysicalDefense: 4},
	)

	if !outcome.IsHit || outcome.Damage != 11 {
		t.Fatalf("expected normal attack to deal averaged damage minus defense, got %+v", outcome)
	}
}

func TestNormalAttackDealsAtLeastOneDamage(t *testing.T) {
	outcome := NormalAttack(
		SnapshotStat{PhysicalAttackMin: 1, PhysicalAttackMax: 1},
		SnapshotStat{PhysicalDefense: 999},
	)

	if outcome.Damage != 1 {
		t.Fatalf("expected minimum damage to be 1, got %+v", outcome)
	}
}

func TestLoadEquipmentConfigsFromDirectory(t *testing.T) {
	dir := t.TempDir()
	weaponDir := filepath.Join(dir, "weapon")
	ringDir := filepath.Join(dir, "ring")
	if err := os.MkdirAll(weaponDir, 0o755); err != nil {
		t.Fatalf("mkdir weapon dir: %v", err)
	}
	if err := os.MkdirAll(ringDir, 0o755); err != nil {
		t.Fatalf("mkdir ring dir: %v", err)
	}

	weaponPath := filepath.Join(weaponDir, "sword.json")
	ringPath := filepath.Join(ringDir, "basic.json")

	if err := os.WriteFile(weaponPath, []byte(`{
  "bronze_sword": {
    "id": "bronze_sword",
    "name": "Bronze Sword",
    "slot": "weapon_main"
  }
}`), 0o600); err != nil {
		t.Fatalf("write weapon config: %v", err)
	}
	if err := os.WriteFile(ringPath, []byte(`{
  "silver_ring": {
    "id": "silver_ring",
    "name": "Silver Ring",
    "slot": "ring"
  }
}`), 0o600); err != nil {
		t.Fatalf("write ring config: %v", err)
	}

	configs, err := LoadEquipmentConfigs(dir)
	if err != nil {
		t.Fatalf("load directory configs: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 equipment configs, got %d", len(configs))
	}
	if configs["bronze_sword"].Slot != "weapon_main" {
		t.Fatalf("expected bronze_sword slot weapon_main, got %+v", configs["bronze_sword"])
	}
	if got := configs["silver_ring"].OccupiesSlots; len(got) != 1 || got[0] != "ring1" {
		t.Fatalf("expected normalized ring occupiesSlots, got %+v", got)
	}
}
