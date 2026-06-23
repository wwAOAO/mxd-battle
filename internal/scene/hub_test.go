package scene

import (
	"testing"
	"time"

	"mxd-battle/internal/combat"
	"mxd-battle/internal/world"
)

type recordingPeer struct {
	events []ServerEvent
}

func (p *recordingPeer) Send(event ServerEvent) {
	p.events = append(p.events, event)
}

func newTestHub(t *testing.T) *Hub {
	t.Helper()

	maps := testRoomMaps()
	hub, err := NewHub(nil, nil, maps)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	return hub
}

func testRoomMaps() map[string]world.MapConfig {
	return map[string]world.MapConfig{
		RoomX: {
			ID:           "wild_x",
			Name:         "Wild Map X",
			Width:        3200,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{ID: "ground_x_1", X1: 0, Y1: 1500, X2: 700, Y2: 1500},
				{ID: "slope_x_1", X1: 700, Y1: 1500, X2: 1100, Y2: 1400},
				{ID: "ground_x_2", X1: 1100, Y1: 1400, X2: 3200, Y2: 1400},
			},
			Platforms: []world.Platform{
				{ID: "floating_stone_x_1", X: 900, Y: 1350, Width: 360, Height: 44},
				{ID: "floating_stone_x_2", X: 900, Y: 1250, Width: 360, Height: 44},
				{ID: "floating_stone_x_3", X: 1600, Y: 1400, Width: 360, Height: 44},
				{ID: "floating_stone_x_4", X: 1700, Y: 1300, Width: 360, Height: 44},
				{ID: "floating_stone_x_5", X: 1820, Y: 1200, Width: 360, Height: 44, SolidSides: true, SolidCeiling: true},
			},
			Walls: []world.Wall{
				{ID: "stone_wall_x_1", X: 1500, Y: 1220, Width: 80, Height: 280},
			},
			Portals: []world.Portal{
				{
					ID:           "x_to_y",
					TargetRoomID: RoomY,
					Area:         world.Rect{X: 3140, Y: 1300, Width: 60, Height: 240},
					Target:       world.Point{X: 120, Y: 1150},
				},
			},
		},
		RoomY: {
			ID:           "wild_y",
			Name:         "Wild Map Y",
			Width:        2600,
			Height:       1400,
			GroundY:      1150,
			Gravity:      2600,
			JumpVelocity: -920,
			MoveSpeed:    400,
			Spawn:        world.Point{X: 120, Y: 1150},
			Portals:      []world.Portal{},
		},
	}
}

func TestHubOnlyAllowsXYRooms(t *testing.T) {
	hub := newTestHub(t)

	_, err := hub.Join("Z", Player{ID: "p1"}, &recordingPeer{})
	if err != ErrInvalidRoom {
		t.Fatalf("expected ErrInvalidRoom, got %v", err)
	}
}

func TestHubJoinMoveLeaveSynchronizesRoom(t *testing.T) {
	hub := newTestHub(t)
	alicePeer := &recordingPeer{}
	bobPeer := &recordingPeer{}

	state, err := hub.Join(RoomX, Player{ID: "alice", X: -10, Y: 9999}, alicePeer)
	if err != nil {
		t.Fatalf("join alice: %v", err)
	}
	if state.Players["alice"].X != DefaultPlayerWidth/2 || state.Players["alice"].Y != state.Map.GroundY {
		t.Fatalf("expected alice position to be clamped, got %+v", state.Players["alice"])
	}
	if state.Players["alice"].Stat != DefaultPlayerStatBundle() {
		t.Fatalf("expected alice default stat, got %+v", state.Players["alice"].Stat)
	}

	if _, err := hub.Join(RoomX, Player{ID: "bob", X: 10, Y: 20}, bobPeer); err != nil {
		t.Fatalf("join bob: %v", err)
	}
	if len(alicePeer.events) != 1 || alicePeer.events[0].Type != "player_joined" {
		t.Fatalf("expected alice to receive bob join, got %+v", alicePeer.events)
	}

	player, ok := hub.Move("bob", 25, 35)
	if !ok {
		t.Fatal("expected bob move to succeed")
	}
	if player.X != DefaultPlayerWidth/2 || player.Y != 20 {
		t.Fatalf("unexpected bob position: %+v", player)
	}
	if len(alicePeer.events) != 2 || alicePeer.events[1].Type != "player_moved" {
		t.Fatalf("expected alice to receive bob move, got %+v", alicePeer.events)
	}

	hub.Leave("bob")
	if len(alicePeer.events) != 3 || alicePeer.events[2].Type != "player_left" {
		t.Fatalf("expected alice to receive bob leave, got %+v", alicePeer.events)
	}
}

func TestHubKeepsRoleStatOnJoin(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"warrior": {
			Name: "Warrior",
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 150},
				MagicAttackMin:    combat.StatPercent{Intelligence: 100},
				MagicAttackMax:    combat.StatPercent{Intelligence: 150},
				PhysicalDefense:   combat.StatPercent{Strength: 50},
				MagicDefense:      combat.StatPercent{Intelligence: 50},
				MoveSpeed:         combat.StatPercent{Agility: 10},
				Evasion:           combat.StatPercent{Agility: 100},
				Accuracy:          combat.StatPercent{Luck: 100},
				CritRate:          combat.StatPercent{Luck: 10},
				CritDamage:        combat.StatPercent{Strength: 20, Luck: 50},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	baseStat := PlayerStat{
		Strength:     12,
		Intelligence: 8,
		Agility:      15,
		Luck:         3,
		HP:           420,
		MP:           70,
		HPMax:        650,
		MPMax:        120,
	}
	bonusStat := PlayerStat{Strength: 3, Luck: 2}
	customStat := PlayerStatBundle{
		Base:  baseStat,
		Bonus: bonusStat,
	}

	state, err := hub.Join(RoomX, Player{ID: "stat-role", Level: 9, Exp: "12345", JobCode: "warrior", Stat: customStat}, &recordingPeer{})
	if err != nil {
		t.Fatalf("join stat-role: %v", err)
	}

	player := state.Players["stat-role"]
	expectedFinal := baseStat
	expectedFinal.Strength += bonusStat.Strength
	expectedFinal.Luck += bonusStat.Luck
	if player.Level != 9 || player.Exp != "12345" || player.Stat.Base != baseStat || player.Stat.Bonus != bonusStat || player.Stat.Final != expectedFinal {
		t.Fatalf("expected role stat to be kept, got %+v", player)
	}
	if player.CombatStat.PhysicalAttackMin != 15 || player.CombatStat.PhysicalAttackMax != 23 {
		t.Fatalf("expected physical attack to be calculated, got %+v", player.CombatStat)
	}
	if player.CombatStat.MagicAttackMin != 8 || player.CombatStat.MagicAttackMax != 12 {
		t.Fatalf("expected magic attack to be calculated, got %+v", player.CombatStat)
	}
	if player.CombatStat.MoveSpeed != 2 || player.CombatStat.CritDamage != 6 {
		t.Fatalf("expected utility stats to be calculated, got %+v", player.CombatStat)
	}
}

func TestHubSetPrimaryStatRecalculatesCombatStat(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"warrior": {
			Name: "Warrior",
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 150},
				MagicAttackMin:    combat.StatPercent{Intelligence: 100},
				MagicAttackMax:    combat.StatPercent{Intelligence: 150},
				PhysicalDefense:   combat.StatPercent{Strength: 50},
				MagicDefense:      combat.StatPercent{Intelligence: 50},
				MoveSpeed:         combat.StatPercent{Agility: 10},
				Evasion:           combat.StatPercent{Agility: 100},
				Accuracy:          combat.StatPercent{Luck: 100},
				CritRate:          combat.StatPercent{Luck: 10},
				CritDamage:        combat.StatPercent{Strength: 20, Luck: 50},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}

	alicePeer := &recordingPeer{}
	bobPeer := &recordingPeer{}
	if _, err := hub.Join(RoomX, Player{ID: "alice", JobCode: "warrior"}, alicePeer); err != nil {
		t.Fatalf("join alice: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "bob",
		JobCode: "warrior",
		Stat: PlayerStatBundle{
			Base:  PlayerStat{Strength: 1, Intelligence: 2, Agility: 3, Luck: 4, HP: 300, MP: 40, HPMax: 500, MPMax: 80},
			Bonus: PlayerStat{Strength: 5},
		},
	}, bobPeer); err != nil {
		t.Fatalf("join bob: %v", err)
	}

	player, ok := hub.SetPrimaryStat("bob", PlayerStat{Strength: 20, Intelligence: 10, Agility: 30, Luck: 8})
	if !ok {
		t.Fatal("expected primary stat update to succeed")
	}

	if player.Stat.Base.Strength != 20 || player.Stat.Base.Intelligence != 10 || player.Stat.Base.Agility != 30 || player.Stat.Base.Luck != 8 {
		t.Fatalf("expected base primary stat to update, got %+v", player.Stat.Base)
	}
	if player.Stat.Base.HP != 300 || player.Stat.Base.MP != 40 || player.Stat.Base.HPMax != 500 || player.Stat.Base.MPMax != 80 {
		t.Fatalf("expected base hp/mp to be preserved, got %+v", player.Stat.Base)
	}
	if player.Stat.Final.Strength != 25 {
		t.Fatalf("expected bonus stat to remain in final stat, got %+v", player.Stat.Final)
	}
	if player.CombatStat.PhysicalAttackMin != 25 || player.CombatStat.PhysicalAttackMax != 38 || player.CombatStat.MagicAttackMin != 10 {
		t.Fatalf("expected combat stat to be recalculated, got %+v", player.CombatStat)
	}
	if len(alicePeer.events) != 2 || alicePeer.events[1].Type != "player_stat_updated" {
		t.Fatalf("expected alice to receive stat update, got %+v", alicePeer.events)
	}
}

func TestHubSetEquipmentAppliesEquipmentStats(t *testing.T) {
	equipments := combat.EquipmentConfigs{
		"bronze_sword": {
			ID:          "bronze_sword",
			Name:        "Bronze Sword",
			Slot:        "weapon",
			Stat:        combat.BaseStat{Strength: 5},
			CombatStat:  combat.EquipmentCombatStat{PhysicalAttackMin: 7, PhysicalAttackMax: 11, AttackStartupMS: -20, AttackRecoveryMS: -30, AttackIntervalMS: -50},
			Requirement: combat.EquipmentRequirement{Strength: 10},
		},
		"swift_boots": {
			ID:          "swift_boots",
			Name:        "Swift Boots",
			Slot:        "shoes",
			Stat:        combat.BaseStat{Agility: 8},
			CombatStat:  combat.EquipmentCombatStat{MoveSpeed: 25, Evasion: 3, AttackActiveMS: -10},
			Requirement: combat.EquipmentRequirement{Agility: 8},
		},
	}
	hub, err := NewHubWithJobsAndEquipment(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"warrior": {
			Name: "Warrior",
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 100},
				MoveSpeed:         combat.StatPercent{Agility: 10},
			},
		},
	}, equipments)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "equipped",
		JobCode: "warrior",
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 10, Agility: 8, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join equipped: %v", err)
	}

	player, _, ok := hub.SetEquipment("equipped", []string{"bronze_sword", "swift_boots"})
	if !ok {
		t.Fatal("expected equipment update to succeed")
	}
	if player.Stat.Equipment.Strength != 5 || player.Stat.Equipment.Agility != 8 {
		t.Fatalf("expected equipment layer to be populated, got %+v", player.Stat.Equipment)
	}
	if player.Stat.Final.Strength != 15 || player.Stat.Final.Agility != 16 {
		t.Fatalf("expected final stat to include equipment bonuses, got %+v", player.Stat.Final)
	}
	if player.CombatStat.PhysicalAttackMin != 22 || player.CombatStat.PhysicalAttackMax != 26 || player.CombatStat.MoveSpeed != 27 || player.CombatStat.Evasion != 3 {
		t.Fatalf("expected combat stat to reflect equipment bonuses, got %+v", player.CombatStat)
	}
	if player.CombatStat.AttackStartupMS != 130 || player.CombatStat.AttackActiveMS != 90 || player.CombatStat.AttackRecoveryMS != 320 || player.CombatStat.AttackIntervalMS != 550 {
		t.Fatalf("expected attack timing stats to include equipment bonuses, got %+v", player.CombatStat)
	}
}

func TestHubSetEquipmentFiltersItemsByRequirement(t *testing.T) {
	equipments := combat.EquipmentConfigs{
		"bronze_sword": {
			ID:          "bronze_sword",
			Name:        "Bronze Sword",
			Slot:        "weapon",
			Stat:        combat.BaseStat{Strength: 5},
			CombatStat:  combat.EquipmentCombatStat{PhysicalAttackMin: 7, PhysicalAttackMax: 11},
			Requirement: combat.EquipmentRequirement{Strength: 10},
		},
		"swift_boots": {
			ID:          "swift_boots",
			Name:        "Swift Boots",
			Slot:        "shoes",
			Stat:        combat.BaseStat{Agility: 8},
			CombatStat:  combat.EquipmentCombatStat{MoveSpeed: 25, Evasion: 3},
			Requirement: combat.EquipmentRequirement{Agility: 8},
		},
	}
	hub, err := NewHubWithJobsAndEquipment(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"warrior": {
			Name: "Warrior",
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 100},
				MoveSpeed:         combat.StatPercent{Agility: 10},
			},
		},
	}, equipments)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "gated",
		JobCode: "warrior",
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 9, Agility: 7, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join gated: %v", err)
	}

	player, _, ok := hub.SetEquipment("gated", []string{"bronze_sword", "swift_boots"})
	if !ok {
		t.Fatal("expected equipment update to succeed")
	}
	if len(player.EquipmentIDs) != 0 {
		t.Fatalf("expected gated player to equip nothing, got %+v", player.EquipmentIDs)
	}
	if player.Stat.Equipment != (PlayerStat{}) {
		t.Fatalf("expected no equipment bonuses when requirements fail, got %+v", player.Stat.Equipment)
	}

	player, ok = hub.SetPrimaryStat("gated", PlayerStat{Strength: 10, Agility: 8})
	if !ok {
		t.Fatal("expected primary stat update to succeed")
	}
	player, _, ok = hub.SetEquipment("gated", []string{"bronze_sword", "swift_boots"})
	if !ok {
		t.Fatal("expected gated equipment update to succeed after stat increase")
	}
	if len(player.EquipmentIDs) != 2 {
		t.Fatalf("expected both equipments to be worn after meeting requirements, got %+v", player.EquipmentIDs)
	}

	player, ok = hub.SetPrimaryStat("gated", PlayerStat{Strength: 9, Agility: 8})
	if !ok {
		t.Fatal("expected stat decrease to succeed")
	}
	if len(player.EquipmentIDs) != 1 || player.EquipmentIDs[0] != "swift_boots" {
		t.Fatalf("expected weapon to be auto-removed after stat decrease, got %+v", player.EquipmentIDs)
	}
	if player.Stat.Equipment.Strength != 0 || player.Stat.Equipment.Agility != 8 {
		t.Fatalf("expected only valid equipment bonuses to remain, got %+v", player.Stat.Equipment)
	}
}

func TestHubSetEquipmentFiltersJobLevelGenderAndExclusiveRules(t *testing.T) {
	equipments := combat.EquipmentConfigs{
		"bronze_sword": {
			ID:          "bronze_sword",
			Name:        "Bronze Sword",
			Slot:        "weapon",
			Stat:        combat.BaseStat{Strength: 5},
			Requirement: combat.EquipmentRequirement{Strength: 10},
			MinLevel:    5,
			AllowedJobs: []string{"warrior"},
		},
		"apprentice_staff": {
			ID:             "apprentice_staff",
			Name:           "Apprentice Staff",
			Slot:           "weapon",
			Stat:           combat.BaseStat{Intelligence: 6},
			Requirement:    combat.EquipmentRequirement{Intelligence: 12},
			MinLevel:       5,
			AllowedJobs:    []string{"mage"},
			AllowedGenders: []string{"female"},
		},
		"lucky_charm": {
			ID:               "lucky_charm",
			Name:             "Lucky Charm",
			Slot:             "accessory",
			Stat:             combat.BaseStat{Luck: 6},
			Requirement:      combat.EquipmentRequirement{Luck: 6},
			ExclusiveGroup:   "trinket",
			IncompatibleWith: []string{"traveler_cloak"},
		},
		"traveler_cloak": {
			ID:               "traveler_cloak",
			Name:             "Traveler Cloak",
			Slot:             "armor",
			Stat:             combat.BaseStat{HPMax: 80},
			Requirement:      combat.EquipmentRequirement{Strength: 5},
			ExclusiveGroup:   "trinket",
			IncompatibleWith: []string{"lucky_charm"},
		},
	}
	hub, err := NewHubWithJobsAndEquipment(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"warrior": {Name: "Warrior"},
		"mage":    {Name: "Mage"},
	}, equipments)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "rule-check",
		JobCode: "warrior",
		Gender:  "male",
		Level:   4,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 10, Intelligence: 12, Luck: 6, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join rule-check: %v", err)
	}

	player, _, ok := hub.SetEquipment("rule-check", []string{"bronze_sword", "apprentice_staff", "lucky_charm", "traveler_cloak"})
	if !ok {
		t.Fatal("expected equipment update to succeed")
	}
	if len(player.EquipmentIDs) != 1 || player.EquipmentIDs[0] != "lucky_charm" {
		t.Fatalf("expected only lucky_charm to remain under initial restrictions, got %+v", player.EquipmentIDs)
	}

	hub.mu.Lock()
	room := hub.rooms[RoomX]
	mutated := room.players["rule-check"]
	mutated.Level = 6
	mutated.JobCode = "mage"
	mutated.Gender = "female"
	room.players["rule-check"] = mutated
	hub.mu.Unlock()

	player, _, ok = hub.SetEquipment("rule-check", []string{"bronze_sword", "apprentice_staff", "lucky_charm", "traveler_cloak"})
	if !ok {
		t.Fatal("expected second equipment update to succeed")
	}
	if len(player.EquipmentIDs) != 2 {
		t.Fatalf("expected two items after relaxed restrictions, got %+v", player.EquipmentIDs)
	}
	if player.EquipmentIDs[0] != "apprentice_staff" {
		t.Fatalf("expected job/gender-valid staff to occupy weapon slot, got %+v", player.EquipmentIDs)
	}
	if player.EquipmentIDs[1] != "lucky_charm" {
		t.Fatalf("expected first trinket-compatible item to remain and block cloak, got %+v", player.EquipmentIDs)
	}
	if player.Stat.Equipment.Intelligence != 6 || player.Stat.Equipment.HPMax != 0 {
		t.Fatalf("expected incompatible cloak to be rejected while charm stays equipped, got %+v", player.Stat.Equipment)
	}
}

func TestHubSetEquipmentSupportsDualRingsAndTwoHandedWeapons(t *testing.T) {
	equipments := combat.EquipmentConfigs{
		"bronze_sword": {
			ID:         "bronze_sword",
			Name:       "Bronze Sword",
			Slot:       "weapon_main",
			Stat:       combat.BaseStat{Strength: 5},
			CombatStat: combat.EquipmentCombatStat{PhysicalAttackMin: 5},
		},
		"wooden_shield": {
			ID:         "wooden_shield",
			Name:       "Wooden Shield",
			Slot:       "weapon_sub",
			Stat:       combat.BaseStat{HPMax: 60},
			CombatStat: combat.EquipmentCombatStat{PhysicalDefense: 10, AttackRecoveryMS: -40},
		},
		"apprentice_staff": {
			ID:         "apprentice_staff",
			Name:       "Apprentice Staff",
			Slot:       "weapon_main",
			SlotCount:  2,
			Stat:       combat.BaseStat{Intelligence: 6},
			CombatStat: combat.EquipmentCombatStat{MagicAttackMin: 10, MagicAttackMax: 16, AttackIntervalMS: -80},
		},
		"silver_ring": {
			ID:         "silver_ring",
			Name:       "Silver Ring",
			Slot:       "ring",
			Stat:       combat.BaseStat{Luck: 4},
			CombatStat: combat.EquipmentCombatStat{CritRate: 2},
		},
		"gold_ring": {
			ID:         "gold_ring",
			Name:       "Gold Ring",
			Slot:       "ring",
			Stat:       combat.BaseStat{Intelligence: 3},
			CombatStat: combat.EquipmentCombatStat{MagicAttackMin: 4, MagicAttackMax: 8},
		},
	}
	hub, err := NewHubWithJobsAndEquipment(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"beginner": {Name: "Beginner"},
	}, equipments)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:    "slot-player",
		Level: 10,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 20, Intelligence: 20, Luck: 10, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join slot-player: %v", err)
	}

	player, selection, ok := hub.SetEquipment("slot-player", []string{"bronze_sword", "wooden_shield", "silver_ring", "gold_ring"})
	if !ok {
		t.Fatal("expected first equipment update to succeed")
	}
	if len(selection.Failures) != 0 {
		t.Fatalf("expected no failures for mainhand+offhand+two rings, got %+v", selection.Failures)
	}
	if len(player.EquipmentIDs) != 4 {
		t.Fatalf("expected four equipped items, got %+v", player.EquipmentIDs)
	}
	if player.Stat.Equipment.Luck != 4 || player.Stat.Equipment.Intelligence != 3 || player.Stat.Equipment.HPMax != 60 {
		t.Fatalf("expected dual rings and offhand stats to apply, got %+v", player.Stat.Equipment)
	}
	if player.CombatStat.PhysicalAttackMin != 5 || player.CombatStat.PhysicalDefense != 10 || player.CombatStat.CritRate != 2 || player.CombatStat.MagicAttackMin != 4 || player.CombatStat.MagicAttackMax != 8 {
		t.Fatalf("expected equipped combat stats to include sword, shield and two rings, got %+v", player.CombatStat)
	}
	if player.CombatStat.AttackRecoveryMS != 310 {
		t.Fatalf("expected shield timing bonus to reduce recovery while equipped, got %+v", player.CombatStat)
	}

	player, selection, ok = hub.SetEquipment("slot-player", []string{"apprentice_staff", "wooden_shield", "silver_ring", "gold_ring"})
	if !ok {
		t.Fatal("expected second equipment update to succeed")
	}
	if len(player.EquipmentIDs) != 3 {
		t.Fatalf("expected two-handed staff to block shield while keeping two rings, got %+v", player.EquipmentIDs)
	}
	if player.EquipmentIDs[0] != "apprentice_staff" {
		t.Fatalf("expected two-handed staff to occupy main weapon slot, got %+v", player.EquipmentIDs)
	}
	foundSlotFailure := false
	for _, failure := range selection.Failures {
		if failure.EquipmentID == "wooden_shield" && failure.Code == "slot_occupied" {
			foundSlotFailure = true
		}
	}
	if !foundSlotFailure {
		t.Fatalf("expected shield rejection reason to be returned, got %+v", selection.Failures)
	}
	if player.Stat.Equipment.HPMax != 0 || player.Stat.Equipment.Intelligence != 9 {
		t.Fatalf("expected staff to apply and shield to be rejected, got %+v", player.Stat.Equipment)
	}
	if player.CombatStat.PhysicalDefense != 0 || player.CombatStat.MagicAttackMin != 14 || player.CombatStat.MagicAttackMax != 24 || player.CombatStat.CritRate != 2 {
		t.Fatalf("expected blocked shield combat stat to be excluded while staff and rings remain, got %+v", player.CombatStat)
	}
	if player.CombatStat.AttackRecoveryMS != 350 || player.CombatStat.AttackIntervalMS != 520 {
		t.Fatalf("expected blocked shield timing bonus to be excluded while staff timing bonus remains, got %+v", player.CombatStat)
	}
}

func TestHubSetPrimaryStatRecalculatesMaxHPAndMP(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			Allocation: combat.CombatStatAllocation{
				HPMax: combat.StatPercent{Strength: 100},
				MPMax: combat.StatPercent{Intelligence: 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID: "growing",
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 10, Intelligence: 20, HP: 100, MP: 40, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join growing: %v", err)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["growing"]
	if player.Stat.Final.HPMax != 510 || player.Stat.Final.MPMax != 120 {
		t.Fatalf("expected max hp/mp to grow from primary stats, got %+v", player.Stat.Final)
	}

	player, ok := hub.SetPrimaryStat("growing", PlayerStat{Strength: 30, Intelligence: 50})
	if !ok {
		t.Fatal("expected primary stat update to succeed")
	}
	if player.Stat.Final.HPMax != 530 || player.Stat.Final.MPMax != 150 {
		t.Fatalf("expected max hp/mp to recalculate after primary stat change, got %+v", player.Stat.Final)
	}
}
func TestHubNormalAttackDamagesPlayerInAttackArea(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"warrior": {
			Name:             "Warrior",
			AttackIntervalMS: 600,
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 100},
				PhysicalDefense:   combat.StatPercent{Strength: 10},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}

	alicePeer := &recordingPeer{}
	bobPeer := &recordingPeer{}
	if _, err := hub.Join(RoomX, Player{
		ID:      "alice",
		JobCode: "warrior",
		X:       500,
		Y:       1500,
		FacingX: 1,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 20, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, alicePeer); err != nil {
		t.Fatalf("join alice: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "bob",
		JobCode: "warrior",
		X:       570,
		Y:       1500,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 10, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, bobPeer); err != nil {
		t.Fatalf("join bob: %v", err)
	}

	result, target, ok := hub.NormalAttack("alice")
	if !ok {
		t.Fatal("expected normal attack to be handled")
	}
	if result.TargetID != "bob" || result.Outcome.Damage != 19 {
		t.Fatalf("expected alice to hit bob for 19 damage, got %+v", result)
	}
	if target.Stat.Final.HP != 481 {
		t.Fatalf("expected bob hp to be reduced, got %+v", target.Stat.Final)
	}
	if len(alicePeer.events) == 0 || alicePeer.events[len(alicePeer.events)-1].Type != "player_attacked" {
		t.Fatalf("expected attack event to be broadcast to alice, got %+v", alicePeer.events)
	}
	if len(bobPeer.events) == 0 || bobPeer.events[len(bobPeer.events)-1].Player == nil {
		t.Fatalf("expected attack event with target player to be broadcast to bob, got %+v", bobPeer.events)
	}
}

func TestHubNormalAttackRespectsAttackInterval(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"warrior": {
			Name:             "Warrior",
			AttackIntervalMS: 600,
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}

	if _, err := hub.Join(RoomX, Player{
		ID:      "alice",
		JobCode: "warrior",
		X:       500,
		Y:       1500,
		FacingX: 1,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 20, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join alice: %v", err)
	}

	now := testTime()
	if _, _, ok := hub.normalAttackAt("alice", now); !ok {
		t.Fatal("expected first attack to be allowed")
	}
	if _, _, ok := hub.normalAttackAt("alice", now.Add(599*time.Millisecond)); ok {
		t.Fatal("expected second attack inside interval to be blocked")
	}
	if _, _, ok := hub.normalAttackAt("alice", now.Add(600*time.Millisecond)); !ok {
		t.Fatal("expected attack at interval boundary to be allowed")
	}
}

func TestHubCastSkillBlockedDuringNormalAttackInterval(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"mage": {
			Name:             "Mage",
			AttackIntervalMS: 600,
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 100},
				MagicAttackMin:    combat.StatPercent{Intelligence: 100},
				MagicAttackMax:    combat.StatPercent{Intelligence: 100},
			},
		},
	}, combat.SkillConfigs{
		"magic_missile": {
			ID:         "magic_missile",
			Name:       "Magic Missile",
			MPCost:     12,
			CooldownMS: 900,
			StartupMS:  300,
			ActiveMS:   80,
			RecoveryMS: 420,
			Projectile: combat.ProjectileConfig{Speed: 760, Width: 48, Height: 32, MaxDistance: 420},
			Damage:     combat.SkillDamageConfig{Base: 8, MagicRate: 1.2},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "alice",
		JobCode: "mage",
		X:       500,
		Y:       1500,
		FacingX: 1,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 20, Intelligence: 30, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join alice: %v", err)
	}

	now := testTime()
	if _, _, ok := hub.normalAttackAt("alice", now); !ok {
		t.Fatal("expected normal attack to be allowed")
	}
	if _, _, ok := hub.castSkillAt("alice", "magic_missile", now.Add(599*time.Millisecond)); ok {
		t.Fatal("expected skill inside normal attack interval to be blocked")
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after blocked skill: %v", err)
	}
	if state.Players["alice"].Stat.Final.MP != 100 {
		t.Fatalf("expected blocked skill not to consume mp, got %+v", state.Players["alice"].Stat.Final)
	}

	if _, _, ok := hub.castSkillAt("alice", "magic_missile", now.Add(600*time.Millisecond)); !ok {
		t.Fatal("expected skill after normal attack interval to be allowed")
	}
}
func TestHubSkillActionLocksNormalAttackAndOtherSkills(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"mage": {
			Name:             "Mage",
			AttackIntervalMS: 600,
			Allocation: combat.CombatStatAllocation{
				PhysicalAttackMin: combat.StatPercent{Strength: 100},
				PhysicalAttackMax: combat.StatPercent{Strength: 100},
				MagicAttackMin:    combat.StatPercent{Intelligence: 100},
				MagicAttackMax:    combat.StatPercent{Intelligence: 100},
			},
		},
	}, combat.SkillConfigs{
		"magic_missile": {
			ID:         "magic_missile",
			Name:       "Magic Missile",
			MPCost:     12,
			CooldownMS: 1200,
			StartupMS:  300,
			ActiveMS:   80,
			RecoveryMS: 420,
			Projectile: combat.ProjectileConfig{Speed: 760, Width: 48, Height: 32, MaxDistance: 420},
			Damage:     combat.SkillDamageConfig{Base: 8, MagicRate: 1.2},
		},
		"ice_bolt": {
			ID:         "ice_bolt",
			Name:       "Ice Bolt",
			MPCost:     8,
			CooldownMS: 500,
			StartupMS:  100,
			ActiveMS:   50,
			RecoveryMS: 150,
			Projectile: combat.ProjectileConfig{Speed: 500, Width: 40, Height: 28, MaxDistance: 300},
			Damage:     combat.SkillDamageConfig{Base: 4, MagicRate: 1},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "alice",
		JobCode: "mage",
		X:       500,
		Y:       1500,
		FacingX: 1,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Strength: 20, Intelligence: 30, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join alice: %v", err)
	}

	now := testTime()
	if _, _, ok := hub.castSkillAt("alice", "magic_missile", now); !ok {
		t.Fatal("expected first skill to be allowed")
	}
	if _, _, ok := hub.normalAttackAt("alice", now.Add(799*time.Millisecond)); ok {
		t.Fatal("expected normal attack during skill action interval to be blocked")
	}
	if _, _, ok := hub.castSkillAt("alice", "ice_bolt", now.Add(799*time.Millisecond)); ok {
		t.Fatal("expected another skill during skill action interval to be blocked")
	}
	if _, _, ok := hub.normalAttackAt("alice", now.Add(800*time.Millisecond)); !ok {
		t.Fatal("expected normal attack after skill action interval to be allowed")
	}
}
func TestHubCastSpeedEquipmentReducesSkillStartupAndActionLock(t *testing.T) {
	equipments := combat.EquipmentConfigs{
		"swift_focus_staff": {
			ID:         "swift_focus_staff",
			Name:       "Swift Focus Staff",
			Slot:       "weapon_main",
			CombatStat: combat.EquipmentCombatStat{CastSpeed: 200},
		},
	}
	hub, err := NewHubWithJobsAndEquipment(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"mage": {
			Name: "Mage",
			Allocation: combat.CombatStatAllocation{
				MagicAttackMin: combat.StatPercent{Intelligence: 100},
				MagicAttackMax: combat.StatPercent{Intelligence: 100},
			},
		},
	}, equipments, combat.SkillConfigs{
		"magic_missile": {
			ID:         "magic_missile",
			Name:       "Magic Missile",
			MPCost:     12,
			CooldownMS: 900,
			StartupMS:  300,
			ActiveMS:   80,
			RecoveryMS: 420,
			Projectile: combat.ProjectileConfig{Speed: 760, Width: 48, Height: 32, MaxDistance: 420},
			Damage:     combat.SkillDamageConfig{Base: 8, MagicRate: 1.2},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:           "alice",
		JobCode:      "mage",
		X:            500,
		Y:            1500,
		FacingX:      1,
		EquipmentIDs: []string{"swift_focus_staff"},
		Stat: PlayerStatBundle{
			Base: PlayerStat{Intelligence: 30, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join alice: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "bob",
		JobCode: "mage",
		X:       650,
		Y:       1500,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Intelligence: 10, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join bob: %v", err)
	}

	now := testTime()
	result, _, ok := hub.castSkillAt("alice", "magic_missile", now)
	if !ok {
		t.Fatal("expected cast to succeed")
	}
	if result.StartupMS != 225 || result.ActiveMS != 60 || result.RecoveryMS != 315 || result.IntervalMS != 600 {
		t.Fatalf("expected skill timing to reflect cast-speed equipment, got %+v", result)
	}

	hub.StepPhysics(now.Add(224*time.Millisecond), 0.1)
	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state before cast startup completes: %v", err)
	}
	if len(state.Projectiles) != 0 {
		t.Fatalf("expected no projectile before reduced startup completes, got %+v", state.Projectiles)
	}
	if _, _, ok := hub.normalAttackAt("alice", now.Add(599*time.Millisecond)); ok {
		t.Fatal("expected normal attack inside reduced skill interval to be blocked")
	}

	hub.StepPhysics(now.Add(225*time.Millisecond), 0.1)
	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after reduced startup completes: %v", err)
	}
	if state.Players["bob"].Stat.Final.HP == 500 {
		t.Fatalf("expected reduced startup to release projectile or apply hit by 225ms, got %+v", state.Players["bob"].Stat.Final)
	}
	if _, _, ok := hub.normalAttackAt("alice", now.Add(600*time.Millisecond)); !ok {
		t.Fatal("expected normal attack after reduced skill interval to be allowed")
	}
}

func TestHubCastMagicMissileConsumesMPDamagesTargetAndRespectsCooldown(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"mage": {
			Name: "Mage",
			Allocation: combat.CombatStatAllocation{
				MagicAttackMin: combat.StatPercent{Intelligence: 100},
				MagicAttackMax: combat.StatPercent{Intelligence: 100},
				MagicDefense:   combat.StatPercent{Intelligence: 10},
			},
		},
	}, combat.SkillConfigs{
		"magic_missile": {
			ID:         "magic_missile",
			Name:       "Magic Missile",
			MPCost:     12,
			CooldownMS: 900,
			StartupMS:  300,
			ActiveMS:   80,
			RecoveryMS: 420,
			Projectile: combat.ProjectileConfig{
				Speed:       760,
				Width:       48,
				Height:      32,
				MaxDistance: 420,
			},
			Damage: combat.SkillDamageConfig{
				Base:      8,
				MagicRate: 1.2,
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}

	if _, err := hub.Join(RoomX, Player{
		ID:      "alice",
		JobCode: "mage",
		X:       500,
		Y:       1500,
		FacingX: 1,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Intelligence: 30, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join alice: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID:      "bob",
		JobCode: "mage",
		X:       650,
		Y:       1500,
		Stat: PlayerStatBundle{
			Base: PlayerStat{Intelligence: 10, HP: 500, MP: 100, HPMax: 500, MPMax: 100},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join bob: %v", err)
	}

	now := testTime()
	result, target, ok := hub.castSkillAt("alice", "magic_missile", now)
	if !ok {
		t.Fatal("expected magic missile cast to succeed")
	}
	if result.ProjectileID != "" || result.TargetID != "" {
		t.Fatalf("expected magic missile to wait for startup before creating projectile, got %+v", result)
	}
	if target.ID != "" {
		t.Fatalf("expected no immediate target before projectile moves, got %+v", target)
	}
	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Players["alice"].Stat.Final.MP != 88 {
		t.Fatalf("expected alice mp to be consumed, got %+v", state.Players["alice"].Stat.Final)
	}
	if len(state.Projectiles) != 0 {
		t.Fatalf("expected no active projectile during skill startup, got %+v", state.Projectiles)
	}

	hub.StepPhysics(now.Add(299*time.Millisecond), 0.1)
	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state before skill startup completes: %v", err)
	}
	if len(state.Projectiles) != 0 {
		t.Fatalf("expected no projectile before startup completes, got %+v", state.Projectiles)
	}

	hub.StepPhysics(now.Add(300*time.Millisecond), 0.1)
	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after skill startup completes: %v", err)
	}
	if state.Players["bob"].Stat.Final.HP != 457 {
		t.Fatalf("expected bob hp to be reduced after projectile starts and hits, got %+v", state.Players["bob"].Stat.Final)
	}
	if len(state.Projectiles) != 0 {
		t.Fatalf("expected projectile to be removed after immediate hit, got %+v", state.Projectiles)
	}

	if _, _, ok := hub.castSkillAt("alice", "magic_missile", now.Add(899*time.Millisecond)); ok {
		t.Fatal("expected magic missile inside cooldown to be blocked")
	}
	if _, _, ok := hub.castSkillAt("alice", "magic_missile", now.Add(900*time.Millisecond)); !ok {
		t.Fatal("expected magic missile at cooldown boundary to be allowed")
	}
}

func TestHubRecoversHPAndMPFromCombatStats(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			Allocation: combat.CombatStatAllocation{
				HPRecovery: combat.StatPercent{Strength: 100},
				MPRecovery: combat.StatPercent{Intelligence: 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID: "recovering",
		Stat: PlayerStatBundle{
			Base: PlayerStat{
				Strength:     10,
				Intelligence: 20,
				HP:           100,
				MP:           40,
				HPMax:        500,
				MPMax:        100,
			},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join recovering: %v", err)
	}
	initialState, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("initial state: %v", err)
	}
	initialPlayer := initialState.Players["recovering"]
	if initialPlayer.CombatStat.HPRecovery != 10 || initialPlayer.CombatStat.MPRecovery != 20 {
		t.Fatalf("expected initial recovery combat stat, got %+v", initialPlayer.CombatStat)
	}

	for i := 0; i < 10; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*100*time.Millisecond), 0.1)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["recovering"]
	if player.Stat.Final.HP != 110 || player.Stat.Final.MP != 60 {
		t.Fatalf("expected hp/mp to recover by combat stats, got %+v", player.Stat.Final)
	}
}

func TestHubRecoveryOnlyTicksOncePerSecond(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			Allocation: combat.CombatStatAllocation{
				HPRecovery: combat.StatPercent{Strength: 100},
				MPRecovery: combat.StatPercent{Intelligence: 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID: "recovering-tick",
		Stat: PlayerStatBundle{
			Base: PlayerStat{
				Strength:     10,
				Intelligence: 20,
				HP:           100,
				MP:           40,
				HPMax:        500,
				MPMax:        100,
			},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join recovering-tick: %v", err)
	}
	initialState, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("initial state: %v", err)
	}
	initialPlayer := initialState.Players["recovering-tick"]
	if initialPlayer.CombatStat.HPRecovery != 10 || initialPlayer.CombatStat.MPRecovery != 20 {
		t.Fatalf("expected initial recovery combat stat, got %+v", initialPlayer.CombatStat)
	}

	for i := 0; i < 9; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*100*time.Millisecond), 0.1)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state before 1s: %v", err)
	}
	player := state.Players["recovering-tick"]
	if player.Stat.Final.HP != 100 || player.Stat.Final.MP != 40 {
		t.Fatalf("expected no recovery before 1 second, got %+v", player.Stat.Final)
	}

	hub.StepPhysics(testTime().Add(900*time.Millisecond), 0.1)
	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state at 1s: %v", err)
	}
	player = state.Players["recovering-tick"]
	if player.Stat.Final.HP != 110 || player.Stat.Final.MP != 60 {
		t.Fatalf("expected one recovery tick at 1 second, got %+v", player.Stat.Final)
	}
}

func TestHubRecoveryCarriesFractionalAmount(t *testing.T) {
	hub, err := NewHubWithJobs(nil, nil, testRoomMaps(), combat.JobStatConfigs{
		"beginner": {
			Name: "Beginner",
			Allocation: combat.CombatStatAllocation{
				HPRecovery: combat.StatPercent{Strength: 40},
				MPRecovery: combat.StatPercent{Intelligence: 40},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{
		ID: "fractional-recovering",
		Stat: PlayerStatBundle{
			Base: PlayerStat{
				Strength:     1,
				Intelligence: 1,
				HP:           100,
				MP:           40,
				HPMax:        500,
				MPMax:        100,
			},
		},
	}, &recordingPeer{}); err != nil {
		t.Fatalf("join fractional-recovering: %v", err)
	}

	for i := 0; i < 20; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*100*time.Millisecond), 0.1)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state before carry reaches 1: %v", err)
	}
	player := state.Players["fractional-recovering"]
	if player.Stat.Final.HP != 100 || player.Stat.Final.MP != 40 {
		t.Fatalf("expected fractional recovery to be carried without visible hp/mp change, got %+v", player.Stat.Final)
	}

	for i := 20; i < 30; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*100*time.Millisecond), 0.1)
	}
	state, err = hub.State(RoomX)
	if err != nil {
		t.Fatalf("state after carry reaches 1: %v", err)
	}
	player = state.Players["fractional-recovering"]
	if player.Stat.Final.HP != 101 || player.Stat.Final.MP != 41 {
		t.Fatalf("expected fractional recovery carry to restore one point, got %+v", player.Stat.Final)
	}
}
func TestHubMoveThroughPortalAreaRequiresUsePortal(t *testing.T) {
	hub := newTestHub(t)
	peer := &recordingPeer{}

	if _, err := hub.Join(RoomX, Player{ID: "traveler", X: 3050, Y: 1500}, peer); err != nil {
		t.Fatalf("join traveler: %v", err)
	}

	player, ok := hub.Move("traveler", 3190, 1500)
	if !ok {
		t.Fatal("expected traveler move to succeed")
	}
	if player.Room != RoomX {
		t.Fatalf("expected traveler to stay in room X before using portal, got %+v", player)
	}
	if len(peer.events) != 0 {
		t.Fatalf("expected no portal snapshot before using portal, got %+v", peer.events)
	}

	player, ok = hub.UsePortal("traveler")
	if !ok {
		t.Fatal("expected traveler to use portal")
	}
	if player.Room != RoomY {
		t.Fatalf("expected traveler to transfer to room Y, got %+v", player)
	}
	if player.X != 120 || player.Y != 1150 {
		t.Fatalf("expected traveler at Y target portal point, got %+v", player)
	}
	if len(peer.events) != 1 || peer.events[0].Type != "snapshot" || peer.events[0].Room != RoomY {
		t.Fatalf("expected traveler to receive new room snapshot, got %+v", peer.events)
	}
}

func TestHubAppliesGravityToAirbornePlayer(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "jumper"}, &recordingPeer{}); err != nil {
		t.Fatalf("join jumper: %v", err)
	}

	player, ok := hub.Jump("jumper")
	if !ok {
		t.Fatal("expected jump to succeed")
	}
	if player.OnGround || player.VY >= 0 {
		t.Fatalf("expected jump to launch player upward, got %+v", player)
	}

	hub.StepPhysics(testTime(), 0.1)
	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player = state.Players["jumper"]
	if player.Y >= state.Map.GroundY {
		t.Fatalf("expected gravity to increase y, got %+v", player)
	}
}

func TestHubLandsOnFloatingPlatform(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "stone-runner", X: 980, Y: 900}, &recordingPeer{}); err != nil {
		t.Fatalf("join stone-runner: %v", err)
	}

	for i := 0; i < 10; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*50*time.Millisecond), 0.05)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["stone-runner"]
	if player.Y != 1250 || !player.OnGround {
		t.Fatalf("expected player to land on floating platform, got %+v", player)
	}
}

func TestHubDropsThroughFloatingPlatform(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "dropper", X: 980, Y: 1250}, &recordingPeer{}); err != nil {
		t.Fatalf("join dropper: %v", err)
	}

	player, ok := hub.Drop("dropper")
	if !ok {
		t.Fatal("expected drop to succeed")
	}
	if player.OnGround || player.Y <= 1250 {
		t.Fatalf("expected player to start dropping through platform, got %+v", player)
	}

	for i := 0; i < 10; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*50*time.Millisecond), 0.05)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player = state.Players["dropper"]
	if player.Y <= 1250 {
		t.Fatalf("expected player below original platform after drop, got %+v", player)
	}
}

func TestHubBlocksHorizontalMovementThroughPlatformSide(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "platform-left", X: 1780, Y: 1230}, &recordingPeer{}); err != nil {
		t.Fatalf("join platform-left: %v", err)
	}

	player, ok := hub.Move("platform-left", 1870, 1230)
	if !ok {
		t.Fatal("expected platform-left move to succeed")
	}
	if player.X >= 1820-DefaultPlayerWidth/2 {
		t.Fatalf("expected platform side to block right movement, got %+v", player)
	}

	if _, err := hub.Join(RoomX, Player{ID: "platform-right", X: 2220, Y: 1230}, &recordingPeer{}); err != nil {
		t.Fatalf("join platform-right: %v", err)
	}

	player, ok = hub.Move("platform-right", 2100, 1230)
	if !ok {
		t.Fatal("expected platform-right move to succeed")
	}
	if player.X <= 2180+DefaultPlayerWidth/2 {
		t.Fatalf("expected platform side to block left movement, got %+v", player)
	}
}

func TestHubAllowsHorizontalMovementThroughPlatformSideWhenDisabled(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "one-way-side", X: 860, Y: 1400}, &recordingPeer{}); err != nil {
		t.Fatalf("join one-way-side: %v", err)
	}

	player, ok := hub.Move("one-way-side", 950, 1400)
	if !ok {
		t.Fatal("expected one-way-side move to succeed")
	}
	if player.X != 950 {
		t.Fatalf("expected platform without solidSides to allow side movement, got %+v", player)
	}
}

func TestHubBlocksHeadMovementThroughPlatformCeiling(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "head-blocked", X: 1900, Y: 1360}, &recordingPeer{}); err != nil {
		t.Fatalf("join head-blocked: %v", err)
	}

	room := hub.rooms[RoomX]
	player := room.players["head-blocked"]
	player.Y = 1330
	player.VY = -500
	player = resolvePlayerMovementFrom(room, player, player.X, 1360, testTime()).Player

	expectedY := 1200 + 44 + DefaultPlayerHeight + wallSkin
	if player.Y != expectedY || player.VY != 0 {
		t.Fatalf("expected platform ceiling to block upward movement at y=%v, got %+v", expectedY, player)
	}
}

func TestHubAllowsHeadMovementThroughPlatformCeilingWhenDisabled(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_platform_ceiling_disabled",
			Width:        2000,
			Height:       2000,
			GroundY:      2000,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 2000},
			Platforms: []world.Platform{
				{
					ID:     "one_way_ceiling",
					X:      900,
					Y:      1250,
					Width:  360,
					Height: 44,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "head-allowed", X: 980, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join head-allowed: %v", err)
	}

	room := hub.rooms[RoomX]
	player := room.players["head-allowed"]
	player.Y = 1480
	player.VY = -500
	player = resolvePlayerMovementFrom(room, player, player.X, 1500, testTime()).Player

	if player.Y != 1480 || player.VY != -500 {
		t.Fatalf("expected platform without solidCeiling to allow upward movement, got %+v", player)
	}
}

func TestHubDoesNotDropThroughGround(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "grounded"}, &recordingPeer{}); err != nil {
		t.Fatalf("join grounded: %v", err)
	}

	player, ok := hub.Drop("grounded")
	if !ok {
		t.Fatal("expected drop request to be handled")
	}
	if !player.OnGround || player.Y != terrainLandingY(hub.rooms[RoomX].mapDef, player) {
		t.Fatalf("expected ground drop to leave player grounded, got %+v", player)
	}
}

func TestHubLandsOnTerrainSlope(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "slope-runner", X: 800, Y: 1300}, &recordingPeer{}); err != nil {
		t.Fatalf("join slope-runner: %v", err)
	}

	for i := 0; i < 10; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*50*time.Millisecond), 0.05)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["slope-runner"]
	if player.Y != 1475 || !player.OnGround {
		t.Fatalf("expected player to land on terrain slope, got %+v", player)
	}
}

func TestHubLandsOnMultiPointTerrain(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_multi_point_terrain",
			Width:        1600,
			Height:       1600,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "ridge",
					Points: []world.Point{
						{X: 0, Y: 1500},
						{X: 500, Y: 1400},
						{X: 1000, Y: 1450},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "terrain-runner", X: 750, Y: 1200}, &recordingPeer{}); err != nil {
		t.Fatalf("join terrain-runner: %v", err)
	}

	for i := 0; i < 10; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*50*time.Millisecond), 0.05)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["terrain-runner"]
	if player.Y != 1425 || !player.OnGround {
		t.Fatalf("expected player to land on multi-point terrain at y=1425, got %+v", player)
	}
}
func TestHubBlocksHorizontalMovementThroughTerrainSideByDefault(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_terrain_side_default",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "step",
					Points: []world.Point{
						{X: 0, Y: 1500},
						{X: 500, Y: 1500},
						{X: 500, Y: 1300},
						{X: 1000, Y: 1300},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "terrain-left", X: 460, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join terrain-left: %v", err)
	}

	player, ok := hub.Move("terrain-left", 560, 1500)
	if !ok {
		t.Fatal("expected terrain-left move to succeed")
	}
	if player.X >= 500-DefaultPlayerWidth/2 {
		t.Fatalf("expected terrain side to block right movement, got %+v", player)
	}
}

func dangerousWoodsTestMap() world.MapConfig {
	return world.MapConfig{
		ID:           "Dangerous_Woods",
		Width:        2400,
		Height:       1500,
		GroundY:      1500,
		Gravity:      2600,
		JumpVelocity: -980,
		MoveSpeed:    420,
		Spawn:        world.Point{X: 180, Y: 1500},
		Terrain: []world.Terrain{
			{
				ID: "terrain_1",
				Points: []world.Point{
					{X: -4, Y: 998},
					{X: 795, Y: 995},
					{X: 793, Y: 1128},
					{X: 1607, Y: 1128},
					{X: 1602, Y: 1002},
					{X: 1806, Y: 1002},
					{X: 1802, Y: 899},
					{X: 2405, Y: 899},
				},
				SolidSides:   world.BoolPtr(true),
				SolidCeiling: world.BoolPtr(false),
			},
		},
	}
}
func TestHubDangerousWoodsAllowsWalkingFromHighSideToLowSide(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: dangerousWoodsTestMap(),
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "dangerous-drop", X: 760, Y: 995}, &recordingPeer{}); err != nil {
		t.Fatalf("join dangerous-drop: %v", err)
	}

	player, ok := hub.Move("dangerous-drop", 830, 995)
	if !ok {
		t.Fatal("expected dangerous-drop move to succeed")
	}
	if player.X != 830 || player.OnGround {
		t.Fatalf("expected high-side-to-low-side move to leave terrain and start falling, got %+v", player)
	}
}
func TestHubDangerousWoodsSingleJumpClearsTerrainStep(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: dangerousWoodsTestMap(),
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "dangerous-jumper", X: 883, Y: 1128}, &recordingPeer{}); err != nil {
		t.Fatalf("join dangerous-jumper: %v", err)
	}
	if _, ok := hub.SetInput("dangerous-jumper", -1, 0); !ok {
		t.Fatal("expected input to be set")
	}
	if _, ok := hub.Jump("dangerous-jumper"); !ok {
		t.Fatal("expected jump to succeed")
	}

	for i := 0; i < 18; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*50*time.Millisecond), 0.05)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["dangerous-jumper"]
	if player.X >= 793-DefaultPlayerWidth/2 || player.Y >= 1128 {
		t.Fatalf("expected one jump while moving left to clear terrain step, got %+v", player)
	}
}
func TestHubBlocksDangerousWoodsLowToHighTerrainMove(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: dangerousWoodsTestMap(),
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "dangerous-low", X: 883, Y: 1112}, &recordingPeer{}); err != nil {
		t.Fatalf("join dangerous-low: %v", err)
	}

	player, ok := hub.Move("dangerous-low", 650, 989)
	if !ok {
		t.Fatal("expected dangerous-low move to succeed")
	}
	if player.X < 793+DefaultPlayerWidth/2 || player.Y < 1100 {
		t.Fatalf("expected low-to-high terrain move to stay blocked below, got %+v", player)
	}
}
func TestHubDoesNotSnapUpTerrainVerticalSideFromBelow(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_terrain_vertical_side_no_snap",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "step",
					Points: []world.Point{
						{X: 0, Y: 1300},
						{X: 500, Y: 1300},
						{X: 500, Y: 1500},
						{X: 1000, Y: 1500},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "low-side", X: 540, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join low-side: %v", err)
	}

	player, ok := hub.Move("low-side", 526, 1500)
	if !ok {
		t.Fatal("expected low-side move to succeed")
	}
	if player.X < 526 || player.Y != 1500 {
		t.Fatalf("expected player to stay below when only the left edge reaches upper terrain, got %+v", player)
	}
}
func TestHubDoesNotSnapUpTerrainNearVerticalSideFromBelow(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_terrain_near_vertical_side_no_snap",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "near_vertical_step",
					Points: []world.Point{
						{X: 0, Y: 1300},
						{X: 500, Y: 1300},
						{X: 504, Y: 1500},
						{X: 1000, Y: 1500},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "near-vertical-low", X: 540, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join near-vertical-low: %v", err)
	}

	player, ok := hub.Move("near-vertical-low", 526, 1500)
	if !ok {
		t.Fatal("expected near-vertical-low move to succeed")
	}
	if player.Y != 1500 {
		t.Fatalf("expected near-vertical terrain side to not snap player upward, got %+v", player)
	}
}
func TestHubBlocksHorizontalMovementThroughTerrainSideFromRight(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_terrain_side_from_right",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "step",
					Points: []world.Point{
						{X: 0, Y: 1300},
						{X: 500, Y: 1300},
						{X: 500, Y: 1500},
						{X: 1000, Y: 1500},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "terrain-right", X: 540, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join terrain-right: %v", err)
	}

	player, ok := hub.Move("terrain-right", 440, 1500)
	if !ok {
		t.Fatal("expected terrain-right move to succeed")
	}
	if player.X <= 500+DefaultPlayerWidth/2 {
		t.Fatalf("expected terrain side to block left movement, got %+v", player)
	}
}
func TestHubAllowsHorizontalMovementThroughTerrainSideWhenDisabled(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_terrain_side_disabled",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID:         "step",
					SolidSides: world.BoolPtr(false),
					Points: []world.Point{
						{X: 0, Y: 1500},
						{X: 500, Y: 1500},
						{X: 500, Y: 1300},
						{X: 1000, Y: 1300},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "terrain-free", X: 460, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join terrain-free: %v", err)
	}

	player, ok := hub.Move("terrain-free", 560, 1500)
	if !ok {
		t.Fatal("expected terrain-free move to succeed")
	}
	if player.X != 560 {
		t.Fatalf("expected terrain with solidSides=false to allow side movement, got %+v", player)
	}
}
func TestHubBlocksWalkingUpSteepTerrainSide(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_steep_terrain_side",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "steep_step",
					Points: []world.Point{
						{X: 0, Y: 1500},
						{X: 500, Y: 1500},
						{X: 650, Y: 1300},
						{X: 1000, Y: 1300},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "steep-runner", X: 480, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join steep-runner: %v", err)
	}

	player, ok := hub.Move("steep-runner", 700, 1500)
	if !ok {
		t.Fatal("expected steep-runner move to succeed")
	}
	if player.X >= 650-DefaultPlayerWidth/2 {
		t.Fatalf("expected steep terrain side to block walking up without jumping, got %+v", player)
	}
}
func TestHubAllowsWalkingUpWalkableTerrainSlope(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_walkable_terrain_slope",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "walkable_slope",
					Points: []world.Point{
						{X: 0, Y: 1500},
						{X: 500, Y: 1500},
						{X: 760, Y: 1380},
						{X: 1000, Y: 1380},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "slope-walker", X: 480, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join slope-walker: %v", err)
	}

	player, ok := hub.Move("slope-walker", 700, 1500)
	if !ok {
		t.Fatal("expected slope-walker move to succeed")
	}
	if player.X != 700 || player.Y >= 1500 {
		t.Fatalf("expected walkable slope to allow movement upward, got %+v", player)
	}
}
func TestHubAllowsWalkingOffTerrainLedge(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_terrain_ledge_drop",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "ledge",
					Points: []world.Point{
						{X: 0, Y: 1300},
						{X: 500, Y: 1300},
						{X: 500, Y: 1500},
						{X: 900, Y: 1500},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "ledge-walker", X: 460, Y: 1300}, &recordingPeer{}); err != nil {
		t.Fatalf("join ledge-walker: %v", err)
	}

	player, ok := hub.Move("ledge-walker", 560, 1300)
	if !ok {
		t.Fatal("expected ledge-walker move to succeed")
	}
	if player.X != 560 || player.OnGround {
		t.Fatalf("expected player to walk off terrain ledge and start falling, got %+v", player)
	}
}
func TestHubKeepsPlayerSupportedUntilFullyPastTerrainLedge(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_terrain_ledge_support",
			Width:        1800,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Terrain: []world.Terrain{
				{
					ID: "ledge",
					Points: []world.Point{
						{X: 0, Y: 1300},
						{X: 500, Y: 1300},
						{X: 500, Y: 1500},
						{X: 900, Y: 1500},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "ledge-edge", X: 460, Y: 1300}, &recordingPeer{}); err != nil {
		t.Fatalf("join ledge-edge: %v", err)
	}

	player, ok := hub.Move("ledge-edge", 520, 1300)
	if !ok {
		t.Fatal("expected ledge-edge move to succeed")
	}
	if player.X != 520 || player.Y != 1300 || !player.OnGround {
		t.Fatalf("expected player to stay supported while foot still overlaps ledge, got %+v", player)
	}
}
func TestHubBlocksHorizontalMovementThroughWall(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "blocked-left", X: 1460, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join blocked-left: %v", err)
	}

	player, ok := hub.Move("blocked-left", 1600, 1500)
	if !ok {
		t.Fatal("expected blocked-left move to succeed")
	}
	if player.X >= 1500-DefaultPlayerWidth/2 {
		t.Fatalf("expected wall to block right movement, got %+v", player)
	}

	if _, err := hub.Join(RoomX, Player{ID: "blocked-right", X: 1620, Y: 1500}, &recordingPeer{}); err != nil {
		t.Fatalf("join blocked-right: %v", err)
	}

	player, ok = hub.Move("blocked-right", 1450, 1500)
	if !ok {
		t.Fatal("expected blocked-right move to succeed")
	}
	if player.X <= 1580+DefaultPlayerWidth/2 {
		t.Fatalf("expected wall to block left movement, got %+v", player)
	}
}

func TestHubLandsOnWallTop(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "wall-lander", X: 1540, Y: 1120}, &recordingPeer{}); err != nil {
		t.Fatalf("join wall-lander: %v", err)
	}

	for i := 0; i < 10; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*50*time.Millisecond), 0.05)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["wall-lander"]
	if player.Y != 1220 || !player.OnGround {
		t.Fatalf("expected player to land on wall top, got %+v", player)
	}
}

func TestHubBlocksHeadMovementThroughWallCeiling(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_wall_ceiling",
			Width:        2000,
			Height:       1500,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Walls: []world.Wall{
				{
					ID:     "floating_wall",
					X:      700,
					Y:      1000,
					Width:  200,
					Height: 120,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "wall-head-blocked", X: 800, Y: 1250}, &recordingPeer{}); err != nil {
		t.Fatalf("join wall-head-blocked: %v", err)
	}

	room := hub.rooms[RoomX]
	player := room.players["wall-head-blocked"]
	player.Y = 1210
	player.VY = -500
	player = resolvePlayerMovementFrom(room, player, player.X, 1250, testTime()).Player

	expectedY := 1000 + 120 + DefaultPlayerHeight + wallSkin
	if player.Y != expectedY || player.VY != 0 {
		t.Fatalf("expected wall ceiling to block upward movement at y=%v, got %+v", expectedY, player)
	}
}

func TestHubClampsMoveSpeedToMaxLimit(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_move_speed_clamp",
			Width:        3200,
			Height:       1800,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    9999,
			Spawn:        world.Point{X: 180, Y: 1500},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "speed-clamped"}, &recordingPeer{}); err != nil {
		t.Fatalf("join speed-clamped: %v", err)
	}

	initialState, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("initial state: %v", err)
	}
	initialX := initialState.Players["speed-clamped"].X

	if _, ok := hub.SetInput("speed-clamped", 1, 0); !ok {
		t.Fatal("expected input to be accepted")
	}
	hub.StepPhysics(testTime(), 0.1)

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["speed-clamped"]
	expectedX := initialX + DefaultMaxPlayerMoveSpeed*0.1
	if player.X != expectedX {
		t.Fatalf("expected clamped move speed to move player to %v, got %+v", expectedX, player)
	}
}

func TestHubMovesHorizontallyFromServerInput(t *testing.T) {
	hub := newTestHub(t)
	if _, err := hub.Join(RoomX, Player{ID: "runner"}, &recordingPeer{}); err != nil {
		t.Fatalf("join runner: %v", err)
	}

	initialState, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("initial state: %v", err)
	}
	initialX := initialState.Players["runner"].X

	if _, ok := hub.SetInput("runner", 1, 0); !ok {
		t.Fatal("expected input to be accepted")
	}
	hub.StepPhysics(testTime(), 0.1)

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["runner"]
	expectedX := initialX + state.Map.MoveSpeed*0.1
	if player.X != expectedX {
		t.Fatalf("expected server speed to move player to %v, got %+v", expectedX, player)
	}
}

func TestHubClimbsLadderFromVerticalInput(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_ladder_climb",
			Width:        1600,
			Height:       1600,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Ladders: []world.Ladder{
				{ID: "ladder_1", X: 300, Y: 1100, Width: 40, Height: 400, ClimbSpeed: 240},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "climber", X: 320, Y: 1450}, &recordingPeer{}); err != nil {
		t.Fatalf("join climber: %v", err)
	}

	player, ok := hub.SetInput("climber", 0, -1)
	if !ok {
		t.Fatal("expected ladder input to be accepted")
	}
	if !player.OnLadder || player.LadderID != "ladder_1" {
		t.Fatalf("expected player to attach to ladder, got %+v", player)
	}

	hub.StepPhysics(testTime(), 0.1)
	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player = state.Players["climber"]
	if !player.OnLadder || player.Y >= 1450 {
		t.Fatalf("expected player to climb upward on ladder, got %+v", player)
	}
	if player.VY >= 0 {
		t.Fatalf("expected upward ladder velocity, got %+v", player)
	}
}

func TestHubAttachesLadderFromTopWhenPressingDown(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_ladder_top_attach",
			Width:        1600,
			Height:       1600,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Platforms: []world.Platform{
				{ID: "top_platform", X: 260, Y: 1100, Width: 200, Height: 40},
			},
			Ladders: []world.Ladder{
				{ID: "ladder_1", X: 300, Y: 1100, Width: 40, Height: 300, ClimbSpeed: 240},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "top-climber", X: 320, Y: 1100}, &recordingPeer{}); err != nil {
		t.Fatalf("join top-climber: %v", err)
	}

	player, ok := hub.SetInput("top-climber", 0, 1)
	if !ok {
		t.Fatal("expected ladder down input to be accepted")
	}
	if !player.OnLadder || player.LadderID != "ladder_1" {
		t.Fatalf("expected player to attach to ladder from the top, got %+v", player)
	}
}

func TestHubLeavesLadderAndFallsWhenNoLongerAttached(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_ladder_detach",
			Width:        1600,
			Height:       1600,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Ladders: []world.Ladder{
				{ID: "ladder_1", X: 300, Y: 1100, Width: 40, Height: 260, ClimbSpeed: 240},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "faller", X: 320, Y: 1320}, &recordingPeer{}); err != nil {
		t.Fatalf("join faller: %v", err)
	}

	if _, ok := hub.SetInput("faller", 0, -1); !ok {
		t.Fatal("expected ladder input to be accepted")
	}
	hub.StepPhysics(testTime(), 0.1)
	if _, ok := hub.SetInput("faller", 1, 0); !ok {
		t.Fatal("expected horizontal input to be accepted")
	}
	for i := 1; i <= 4; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*100*time.Millisecond), 0.1)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["faller"]
	if player.OnLadder {
		t.Fatalf("expected player to leave ladder after moving away, got %+v", player)
	}
	if player.VY <= 0 {
		t.Fatalf("expected gravity to resume after leaving ladder, got %+v", player)
	}
}

func TestHubLandsOnPolygonTop(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_polygon_top",
			Width:        1600,
			Height:       1600,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Polygons: []world.Polygon{
				{
					ID: "stone_polygon",
					Points: []world.Point{
						{X: 700, Y: 1200},
						{X: 980, Y: 1200},
						{X: 1030, Y: 1320},
						{X: 660, Y: 1320},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "polygon-lander", X: 840, Y: 1050}, &recordingPeer{}); err != nil {
		t.Fatalf("join polygon-lander: %v", err)
	}

	for i := 0; i < 10; i++ {
		hub.StepPhysics(testTime().Add(time.Duration(i)*50*time.Millisecond), 0.05)
	}

	state, err := hub.State(RoomX)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	player := state.Players["polygon-lander"]
	if player.Y != 1200 || !player.OnGround {
		t.Fatalf("expected player to land on polygon top, got %+v", player)
	}
}

func TestHubBlocksHorizontalMovementThroughPolygonSide(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_polygon_side",
			Width:        1600,
			Height:       1600,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Polygons: []world.Polygon{
				{
					ID: "stone_polygon",
					Points: []world.Point{
						{X: 700, Y: 1200},
						{X: 980, Y: 1200},
						{X: 1030, Y: 1500},
						{X: 660, Y: 1500},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "polygon-left", X: 620, Y: 1450}, &recordingPeer{}); err != nil {
		t.Fatalf("join polygon-left: %v", err)
	}

	player, ok := hub.Move("polygon-left", 760, 1450)
	if !ok {
		t.Fatal("expected polygon-left move to succeed")
	}
	if player.X >= 660-DefaultPlayerWidth/2 {
		t.Fatalf("expected polygon side to block right movement, got %+v", player)
	}
}

func TestHubBlocksHeadMovementThroughPolygonCeiling(t *testing.T) {
	hub, err := NewHub(nil, nil, map[string]world.MapConfig{
		RoomX: {
			ID:           "test_polygon_ceiling",
			Width:        1600,
			Height:       1600,
			GroundY:      1500,
			Gravity:      2600,
			JumpVelocity: -980,
			MoveSpeed:    420,
			Spawn:        world.Point{X: 180, Y: 1500},
			Polygons: []world.Polygon{
				{
					ID: "floating_polygon",
					Points: []world.Point{
						{X: 700, Y: 900},
						{X: 950, Y: 900},
						{X: 980, Y: 1040},
						{X: 680, Y: 1040},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if _, err := hub.Join(RoomX, Player{ID: "polygon-head", X: 820, Y: 1180}, &recordingPeer{}); err != nil {
		t.Fatalf("join polygon-head: %v", err)
	}

	room := hub.rooms[RoomX]
	player := room.players["polygon-head"]
	player.Y = 1120
	player.VY = -500
	player = resolvePlayerMovementFrom(room, player, player.X, 1180, testTime()).Player

	expectedY := 1040 + DefaultPlayerHeight + wallSkin
	if player.Y != expectedY || player.VY != 0 {
		t.Fatalf("expected polygon ceiling to block upward movement at y=%v, got %+v", expectedY, player)
	}
}
func testTime() time.Time {
	return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
}
