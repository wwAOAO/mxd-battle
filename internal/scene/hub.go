package scene

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"mxd-battle/internal/combat"
	"mxd-battle/internal/world"
)

const (
	RoomX = "A.X"
	RoomY = "Treasure_Island.city"
)

var (
	ErrInvalidRoom      = errors.New("room is not configured")
	ErrPlayerIDRequired = errors.New("player id is required")
)

type EventPublisher interface {
	PublishBattleEvent(subject string, payload []byte) error
}

type Peer interface {
	Send(event ServerEvent)
}

type RoomState struct {
	ID          string                `json:"id"`
	Map         world.MapConfig       `json:"map"`
	Players     map[string]Player     `json:"players"`
	Projectiles map[string]Projectile `json:"projectiles"`
}

type ServerEvent struct {
	Type         string                    `json:"type"`
	Room         string                    `json:"room,omitempty"`
	Player       *Player                   `json:"player,omitempty"`
	PlayerID     string                    `json:"playerId,omitempty"`
	Attack       *AttackResult             `json:"attack,omitempty"`
	Skill        *SkillResult              `json:"skill,omitempty"`
	Projectile   *Projectile               `json:"projectile,omitempty"`
	ProjectileID string                    `json:"projectileId,omitempty"`
	State        *RoomState                `json:"state,omitempty"`
	Message      string                    `json:"message,omitempty"`
	EquipResult  *EquipmentSelectionResult `json:"equipResult,omitempty"`
	CreatedAt    time.Time                 `json:"createdAt"`
}

type AttackResult struct {
	AttackerID string               `json:"attackerId"`
	TargetID   string               `json:"targetId,omitempty"`
	Area       world.Rect           `json:"area"`
	Outcome    combat.AttackOutcome `json:"outcome"`
}

type SkillResult struct {
	SkillID      string               `json:"skillId"`
	CasterID     string               `json:"casterId"`
	TargetID     string               `json:"targetId,omitempty"`
	ProjectileID string               `json:"projectileId,omitempty"`
	Area         world.Rect           `json:"area"`
	MPCost       int32                `json:"mpCost"`
	CooldownMS   int32                `json:"cooldownMs"`
	StartupMS    int32                `json:"startupMs"`
	ActiveMS     int32                `json:"activeMs"`
	RecoveryMS   int32                `json:"recoveryMs"`
	IntervalMS   int32                `json:"intervalMs"`
	Outcome      combat.AttackOutcome `json:"outcome"`
}

type Hub struct {
	logger    *slog.Logger
	publisher EventPublisher

	mu         sync.RWMutex
	rooms      map[string]*room
	players    map[string]string
	jobs       combat.JobStatConfigs
	skills     combat.SkillConfigs
	equipments combat.EquipmentConfigs
}

func NewHub(logger *slog.Logger, publisher EventPublisher, maps map[string]world.MapConfig) (*Hub, error) {
	return NewHubWithJobs(logger, publisher, maps, combat.DefaultJobStatConfigs())
}

func NewHubWithJobs(logger *slog.Logger, publisher EventPublisher, maps map[string]world.MapConfig, jobs combat.JobStatConfigs, skillConfigs ...combat.SkillConfigs) (*Hub, error) {
	return NewHubWithJobsAndEquipment(logger, publisher, maps, jobs, nil, skillConfigs...)
}

func NewHubWithJobsAndEquipment(logger *slog.Logger, publisher EventPublisher, maps map[string]world.MapConfig, jobs combat.JobStatConfigs, equipmentConfigs combat.EquipmentConfigs, skillConfigs ...combat.SkillConfigs) (*Hub, error) {
	if err := world.ValidateRoomMaps(maps); err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		jobs = combat.DefaultJobStatConfigs()
	}
	skills := combat.DefaultSkillConfigs()
	if len(skillConfigs) > 0 && len(skillConfigs[0]) > 0 {
		skills = skillConfigs[0]
	}

	rooms := make(map[string]*room, len(maps))
	for roomID, mapDef := range maps {
		rooms[roomID] = newRoom(roomID, mapDef)
	}

	return &Hub{
		logger:     logger,
		publisher:  publisher,
		rooms:      rooms,
		players:    make(map[string]string),
		jobs:       jobs,
		skills:     skills,
		equipments: combat.NormalizeEquipmentConfigs(equipmentConfigs),
	}, nil
}

func (h *Hub) Rooms() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rooms := make([]string, 0, len(h.rooms))
	for roomID := range h.rooms {
		rooms = append(rooms, roomID)
	}
	slices.Sort(rooms)
	return rooms
}

func (h *Hub) JobStatConfigs() combat.JobStatConfigs {
	h.mu.RLock()
	defer h.mu.RUnlock()

	configs := make(combat.JobStatConfigs, len(h.jobs))
	for code, config := range h.jobs {
		configs[code] = config
	}
	return configs
}

func (h *Hub) EquipmentConfigs() combat.EquipmentConfigs {
	h.mu.RLock()
	defer h.mu.RUnlock()

	configs := make(combat.EquipmentConfigs, len(h.equipments))
	for id, config := range h.equipments {
		configs[id] = config
	}
	return configs
}

func (h *Hub) DefaultRoom() string {
	rooms := h.Rooms()
	if len(rooms) == 0 {
		return ""
	}
	return rooms[0]
}

func (h *Hub) StartPhysics(ctx context.Context) {
	// Physics runs at 20 ticks per second.
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			dt := now.Sub(last).Seconds()
			last = now
			h.StepPhysics(now.UTC(), dt)
		}
	}
}

func (h *Hub) Join(roomID string, player Player, peer Peer) (RoomState, error) {
	if player.ID == "" {
		return RoomState{}, ErrPlayerIDRequired
	}
	if player.Name == "" {
		player.Name = player.ID
	}

	now := time.Now().UTC()

	var previousRoom *room
	var currentRoom *room
	var previousPlayer Player
	var hadPrevious bool

	h.mu.Lock()
	currentRoom = h.rooms[roomID]
	if currentRoom == nil {
		h.mu.Unlock()
		return RoomState{}, ErrInvalidRoom
	}
	if oldRoomID, ok := h.players[player.ID]; ok {
		previousRoom = h.rooms[oldRoomID]
		previousPlayer, hadPrevious = previousRoom.players[player.ID]
		delete(previousRoom.players, player.ID)
		delete(previousRoom.peers, player.ID)
	}

	player = preparePlayerForRoom(player, currentRoom, h.jobs, h.equipments, now)
	currentRoom.players[player.ID] = player
	currentRoom.peers[player.ID] = peer
	h.players[player.ID] = roomID
	state := roomSnapshot(currentRoom)
	joinRecipients := roomPeersExcept(currentRoom, player.ID)
	leaveRecipients := map[string]Peer(nil)
	if previousRoom != nil && previousRoom.id != currentRoom.id {
		leaveRecipients = roomPeersExcept(previousRoom, player.ID)
	}
	h.mu.Unlock()

	if hadPrevious && previousRoom != nil && previousRoom.id != currentRoom.id {
		h.broadcast(leaveRecipients, ServerEvent{
			Type:      "player_left",
			Room:      previousRoom.id,
			PlayerID:  previousPlayer.ID,
			CreatedAt: now,
		})
	}

	h.broadcast(joinRecipients, ServerEvent{
		Type:      "player_joined",
		Room:      roomID,
		Player:    &player,
		CreatedAt: now,
	})
	h.publish("battle.events.world.player_joined", ServerEvent{
		Type:      "player_joined",
		Room:      roomID,
		Player:    &player,
		CreatedAt: now,
	})

	return state, nil
}

func (h *Hub) Move(playerID string, x float64, y float64) (Player, bool) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return Player{}, false
	}
	currentRoom := h.rooms[roomID]
	player := currentRoom.players[playerID]
	player.X = x
	player.UpdatedAt = now

	transition := resolvePlayerMovementFromWithJobs(currentRoom, player, currentRoom.players[playerID].X, player.Y, h.jobs, h.equipments, time.Time{})
	currentRoom.players[playerID] = transition.Player
	recipients := roomPeersExcept(currentRoom, playerID)
	h.mu.Unlock()

	player = transition.Player
	event := ServerEvent{
		Type:      "player_moved",
		Room:      currentRoom.id,
		Player:    &player,
		CreatedAt: now,
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_moved", event)
	return player, true
}

func (h *Hub) UsePortal(playerID string) (Player, bool) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return Player{}, false
	}
	currentRoom := h.rooms[roomID]
	peer := currentRoom.peers[playerID]
	player := currentRoom.players[playerID]
	player, targetRoomID, ok := resolvePortal(currentRoom, player)
	if !ok {
		h.mu.Unlock()
		return player, false
	}

	delete(currentRoom.players, playerID)
	delete(currentRoom.peers, playerID)

	targetRoom := h.rooms[targetRoomID]
	if targetRoom == nil {
		h.mu.Unlock()
		return player, false
	}
	player.MapID = targetRoom.mapDef.ID
	player = applyGround(targetRoom.mapDef, player, player.Y, time.Time{})
	player, _ = NormalizeEquipmentSelection(player, h.equipments)
	player = ApplyEquipmentStats(player, h.equipments)
	player = NormalizePlayerStatWithJobs(player, h.jobs)
	player = applySnapshotStat(player, h.jobs, h.equipments)
	player.UpdatedAt = now
	targetRoom.players[playerID] = player
	targetRoom.peers[playerID] = peer
	h.players[playerID] = targetRoom.id

	oldRecipients := roomPeersExcept(currentRoom, playerID)
	newRecipients := roomPeersExcept(targetRoom, playerID)
	state := roomSnapshot(targetRoom)
	h.mu.Unlock()

	leftEvent := ServerEvent{
		Type:      "player_left",
		Room:      currentRoom.id,
		PlayerID:  playerID,
		CreatedAt: now,
	}
	joinedEvent := ServerEvent{
		Type:      "player_joined",
		Room:      targetRoom.id,
		Player:    &player,
		CreatedAt: now,
	}

	h.broadcast(oldRecipients, leftEvent)
	h.broadcast(newRecipients, joinedEvent)
	peer.Send(ServerEvent{
		Type:      "snapshot",
		Room:      targetRoom.id,
		State:     &state,
		CreatedAt: now,
	})
	h.publish("battle.events.world.player_left", leftEvent)
	h.publish("battle.events.world.player_joined", joinedEvent)
	return player, true
}

func (h *Hub) Jump(playerID string) (Player, bool) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return Player{}, false
	}
	room := h.rooms[roomID]
	player := room.players[playerID]
	if player.OnLadder {
		player = detachLadder(player)
	}
	if player.OnGround {
		player.VY = room.mapDef.JumpVelocity
		player.OnGround = false
		player.UpdatedAt = now
		room.players[playerID] = player
	}
	recipients := roomPeersExcept(room, playerID)
	h.mu.Unlock()

	event := ServerEvent{
		Type:      "player_moved",
		Room:      roomID,
		Player:    &player,
		CreatedAt: now,
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_moved", event)
	return player, true
}

func (h *Hub) Drop(playerID string) (Player, bool) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return Player{}, false
	}
	room := h.rooms[roomID]
	player := room.players[playerID]
	if player.OnGround && isOnDropThroughPlatform(room.mapDef, player) {
		player.DropUntil = now.Add(350 * time.Millisecond)
		player.OnGround = false
		player.VY = 1
		player.Y += 1
		player.UpdatedAt = now
		room.players[playerID] = player
	}
	recipients := roomPeersExcept(room, playerID)
	h.mu.Unlock()

	event := ServerEvent{
		Type:      "player_moved",
		Room:      roomID,
		Player:    &player,
		CreatedAt: now,
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_moved", event)
	return player, true
}

func (h *Hub) SetInput(playerID string, inputX float64, inputY float64) (Player, bool) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return Player{}, false
	}
	room := h.rooms[roomID]
	player := room.players[playerID]
	player.InputX = world.Clamp(inputX, -1, 1)
	player.InputY = world.Clamp(inputY, -1, 1)
	if player.InputX != 0 {
		player.FacingX = player.InputX
	}
	if player.InputY != 0 {
		player = tryAttachLadder(room.mapDef, player)
	} else if player.OnLadder {
		if player.InputX != 0 {
			player = detachLadder(player)
		} else if _, ok := ladderByID(room.mapDef, player.LadderID); !ok {
			player = detachLadder(player)
		}
	}
	player.UpdatedAt = now
	room.players[playerID] = player
	h.mu.Unlock()

	return player, true
}

func (h *Hub) SetPrimaryStat(playerID string, stat PlayerStat) (Player, bool) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return Player{}, false
	}
	room := h.rooms[roomID]
	player := room.players[playerID]
	player.Stat.Base.Strength = maxInt32(0, stat.Strength)
	player.Stat.Base.Intelligence = maxInt32(0, stat.Intelligence)
	player.Stat.Base.Agility = maxInt32(0, stat.Agility)
	player.Stat.Base.Luck = maxInt32(0, stat.Luck)
	player, _ = NormalizeEquipmentSelection(player, h.equipments)
	player = ApplyEquipmentStats(player, h.equipments)
	player = NormalizePlayerStatWithJobs(player, h.jobs)
	player = applySnapshotStat(player, h.jobs, h.equipments)
	player.UpdatedAt = now
	room.players[playerID] = player
	recipients := roomPeersExcept(room, playerID)
	h.mu.Unlock()

	event := ServerEvent{
		Type:      "player_stat_updated",
		Room:      roomID,
		Player:    &player,
		CreatedAt: now,
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_stat_updated", event)
	return player, true
}

func (h *Hub) SetEquipment(playerID string, equipmentIDs []string) (Player, EquipmentSelectionResult, bool) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return Player{}, EquipmentSelectionResult{}, false
	}
	room := h.rooms[roomID]
	player := room.players[playerID]
	selection := EvaluateEquipmentSelection(player, slices.Clone(equipmentIDs), h.equipments)
	player.EquipmentIDs = selection.EquipmentIDs
	player = ApplyEquipmentStats(player, h.equipments)
	player = NormalizePlayerStatWithJobs(player, h.jobs)
	player = applySnapshotStat(player, h.jobs, h.equipments)
	player.UpdatedAt = now
	room.players[playerID] = player
	recipients := roomPeersExcept(room, playerID)
	h.mu.Unlock()

	event := ServerEvent{
		Type:        "player_stat_updated",
		Room:        roomID,
		Player:      &player,
		EquipResult: &selection,
		CreatedAt:   now,
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_stat_updated", event)
	return player, selection, true
}

func (h *Hub) NormalAttack(playerID string) (AttackResult, Player, bool) {
	return h.normalAttackAt(playerID, time.Now().UTC())
}

func (h *Hub) normalAttackAt(playerID string, now time.Time) (AttackResult, Player, bool) {
	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return AttackResult{}, Player{}, false
	}
	room := h.rooms[roomID]
	attacker := clearExpiredAction(room.players[playerID], now)
	if isActionLocked(attacker, now) {
		h.mu.Unlock()
		return AttackResult{}, Player{}, false
	}
	if !combat.CanNormalAttack(now, attacker.LastAttackAt, attacker.CombatStat) {
		h.mu.Unlock()
		return AttackResult{}, Player{}, false
	}
	attacker.LastAttackAt = now
	attacker.ActionKind = actionKindNormalAttack
	attacker.ActionLockedUntil = now.Add(combat.AttackInterval(attacker.CombatStat))
	attacker.UpdatedAt = now
	room.players[playerID] = attacker

	area := normalAttackArea(attacker)
	result := AttackResult{
		AttackerID: playerID,
		Area:       area,
	}

	var target Player
	for targetID, candidate := range room.players {
		if targetID == playerID {
			continue
		}
		if playerIntersectsRect(candidate, area) {
			outcome := combat.NormalAttack(attacker.CombatStat, candidate.CombatStat)
			candidate.Stat.Base.HP = maxInt32(0, candidate.Stat.Base.HP-outcome.Damage)
			candidate = ApplyEquipmentStats(candidate, h.equipments)
			candidate = NormalizePlayerStatWithJobs(candidate, h.jobs)
			candidate = applySnapshotStat(candidate, h.jobs, h.equipments)
			candidate.UpdatedAt = now
			room.players[targetID] = candidate
			target = candidate
			result.TargetID = targetID
			result.Outcome = outcome
			break
		}
	}

	recipients := roomPeers(room)
	h.mu.Unlock()

	event := ServerEvent{
		Type:      "player_attacked",
		Room:      roomID,
		Attack:    &result,
		CreatedAt: now,
	}
	if result.TargetID != "" {
		event.Player = &target
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_attacked", event)
	return result, target, true
}

func (h *Hub) CastSkill(playerID string, skillID string) (SkillResult, Player, bool) {
	return h.castSkillAt(playerID, skillID, time.Now().UTC())
}

func (h *Hub) castSkillAt(playerID string, skillID string, now time.Time) (SkillResult, Player, bool) {
	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return SkillResult{}, Player{}, false
	}
	skill, ok := h.skills[skillID]
	if !ok {
		h.mu.Unlock()
		return SkillResult{}, Player{}, false
	}

	room := h.rooms[roomID]
	caster := clearExpiredAction(room.players[playerID], now)
	if isActionLocked(caster, now) {
		h.mu.Unlock()
		return SkillResult{}, Player{}, false
	}
	if caster.Stat.Final.MP < skill.MPCost {
		h.mu.Unlock()
		return SkillResult{}, Player{}, false
	}
	if caster.LastSkillAt != nil && !combat.CanCastSkill(now, caster.LastSkillAt[skillID], skill) {
		h.mu.Unlock()
		return SkillResult{}, Player{}, false
	}

	caster.Stat.Base.MP = maxInt32(0, caster.Stat.Base.MP-skill.MPCost)
	if caster.LastSkillAt == nil {
		caster.LastSkillAt = make(map[string]time.Time)
	}
	caster.LastSkillAt[skillID] = now
	caster.ActionKind = actionKindSkill
	caster = ApplyEquipmentStats(caster, h.equipments)
	caster = NormalizePlayerStatWithJobs(caster, h.jobs)
	caster = applySnapshotStat(caster, h.jobs, h.equipments)
	timing := combat.CalculateSkillTiming(skill, caster.CombatStat)
	caster.ActionLockedUntil = now.Add(combat.SkillCastInterval(skill, caster.CombatStat))
	caster.UpdatedAt = now
	room.players[playerID] = caster

	pending := PendingSkill{
		ID:        pendingSkillID(playerID, skillID, now),
		CasterID:  playerID,
		SkillID:   skillID,
		Skill:     skill,
		Timing:    timing,
		ReadyAt:   now.Add(combat.SkillStartup(skill, caster.CombatStat)),
		CreatedAt: now,
	}
	room.pendingSkills[pending.ID] = pending

	result := SkillResult{
		SkillID:    skillID,
		CasterID:   playerID,
		MPCost:     skill.MPCost,
		CooldownMS: skill.CooldownMS,
		StartupMS:  timing.StartupMS,
		ActiveMS:   timing.ActiveMS,
		RecoveryMS: timing.RecoveryMS,
		IntervalMS: timing.IntervalMS,
	}

	recipients := roomPeers(room)
	h.mu.Unlock()

	casterEvent := caster
	event := ServerEvent{
		Type:      "player_skill_started",
		Room:      roomID,
		Player:    &casterEvent,
		Skill:     &result,
		CreatedAt: now,
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_skill_started", event)
	return result, Player{}, true
}
func (h *Hub) StepPhysics(now time.Time, dt float64) {
	if dt <= 0 || dt > 0.25 {
		return
	}

	type queuedEvent struct {
		recipients map[string]Peer
		event      ServerEvent
	}

	var queued []queuedEvent

	h.mu.Lock()
	for _, room := range h.rooms {
		for playerID, player := range room.players {
			player = clearExpiredAction(player, now)
			previousPlayer := player
			recovered := false
			player = applyRecovery(player, dt)
			player = ApplyEquipmentStats(player, h.equipments)
			player = NormalizePlayerStatWithJobs(player, h.jobs)
			player = applySnapshotStat(player, h.jobs, h.equipments)
			room.players[playerID] = player
			if player.Stat.Final.HP != previousPlayer.Stat.Final.HP ||
				player.Stat.Final.MP != previousPlayer.Stat.Final.MP {
				recovered = true
				player.UpdatedAt = now
				room.players[playerID] = player
			}

			if player.OnGround && !player.OnLadder && player.VY == 0 && player.InputX == 0 && player.InputY == 0 {
				if recovered {
					recipients := roomPeers(room)
					playerCopy := player
					queued = append(queued, queuedEvent{
						recipients: recipients,
						event: ServerEvent{
							Type:      "player_recovered",
							Room:      room.id,
							Player:    &playerCopy,
							CreatedAt: now,
						},
					})
				}
				continue
			}

			previousX := player.X
			previousY := player.Y
			moveSpeed := world.Clamp(room.mapDef.MoveSpeed, 0, DefaultMaxPlayerMoveSpeed)
			player.VX = player.InputX * moveSpeed
			player.X += player.VX * dt
			if player.OnLadder && player.InputY == 0 && player.InputX != 0 {
				player = detachLadder(player)
			}
			if player.OnLadder {
				ladder, ok := ladderByID(room.mapDef, player.LadderID)
				if !ok {
					player = detachLadder(player)
				}
				if player.OnLadder {
					player.VX = 0
					player.VY = player.InputY * ladderClimbSpeed(ladder)
					player.Y += player.VY * dt
					player.X = world.Clamp(player.X, ladder.X, ladder.X+ladder.Width)
					if player.Y < ladder.Y {
						player.Y = ladder.Y
					}
					ladderBottom := ladder.Y + ladder.Height
					if player.Y > ladderBottom {
						player.Y = ladderBottom
					}
				}
			}
			if !player.OnLadder {
				player.VY += room.mapDef.Gravity * dt
				player.Y += player.VY * dt
			}
			player.UpdatedAt = now
			player = resolvePlayerMovementFromWithJobs(room, player, previousX, previousY, h.jobs, h.equipments, now).Player
			room.players[playerID] = player

			recipients := roomPeers(room)
			playerCopy := player
			queued = append(queued, queuedEvent{
				recipients: recipients,
				event: ServerEvent{
					Type:      "player_moved",
					Room:      room.id,
					Player:    &playerCopy,
					CreatedAt: now,
				},
			})
		}
		for pendingID, pending := range room.pendingSkills {
			if now.Before(pending.ReadyAt) {
				continue
			}
			delete(room.pendingSkills, pendingID)
			caster, ok := room.players[pending.CasterID]
			if !ok {
				continue
			}

			projectile := newProjectile(caster, pending.Skill, now)
			room.projectiles[projectile.ID] = projectile
			area := projectileRect(projectile)
			result := SkillResult{
				SkillID:      pending.SkillID,
				CasterID:     pending.CasterID,
				ProjectileID: projectile.ID,
				Area:         area,
				MPCost:       pending.Skill.MPCost,
				CooldownMS:   pending.Skill.CooldownMS,
				StartupMS:    pending.Timing.StartupMS,
				ActiveMS:     pending.Timing.ActiveMS,
				RecoveryMS:   pending.Timing.RecoveryMS,
				IntervalMS:   pending.Timing.IntervalMS,
			}
			recipients := roomPeers(room)
			projectileCopy := projectile
			queued = append(queued, queuedEvent{
				recipients: recipients,
				event: ServerEvent{
					Type:       "player_skill_cast",
					Room:       room.id,
					Player:     &caster,
					Skill:      &result,
					Projectile: &projectileCopy,
					CreatedAt:  now,
				},
			})
		}

		for projectileID, projectile := range room.projectiles {
			previousX := projectile.X
			projectile.X += projectile.VX * dt
			projectile.Distance += absFloat64(projectile.X - previousX)
			room.projectiles[projectileID] = projectile

			recipients := roomPeers(room)
			projectileCopy := projectile
			queued = append(queued, queuedEvent{
				recipients: recipients,
				event: ServerEvent{
					Type:       "projectile_moved",
					Room:       room.id,
					Projectile: &projectileCopy,
					CreatedAt:  now,
				},
			})

			var hitTarget Player
			var hitTargetID string
			for targetID, candidate := range room.players {
				if targetID == projectile.CasterID {
					continue
				}
				if playerIntersectsRect(candidate, projectileRect(projectile)) {
					outcome := combat.MagicSkillAttack(projectile.CasterStat, candidate.CombatStat, projectile.Skill)
					candidate.Stat.Base.HP = maxInt32(0, candidate.Stat.Base.HP-outcome.Damage)
					candidate = ApplyEquipmentStats(candidate, h.equipments)
					candidate = NormalizePlayerStatWithJobs(candidate, h.jobs)
					candidate = applySnapshotStat(candidate, h.jobs, h.equipments)
					candidate.UpdatedAt = now
					room.players[targetID] = candidate
					hitTarget = candidate
					hitTargetID = targetID
					result := SkillResult{
						SkillID:      projectile.SkillID,
						CasterID:     projectile.CasterID,
						TargetID:     targetID,
						ProjectileID: projectile.ID,
						Area:         projectileRect(projectile),
						MPCost:       projectile.Skill.MPCost,
						CooldownMS:   projectile.Skill.CooldownMS,
						Outcome:      outcome,
					}
					queued = append(queued, queuedEvent{
						recipients: recipients,
						event: ServerEvent{
							Type:         "player_skill_hit",
							Room:         room.id,
							Player:       &hitTarget,
							Skill:        &result,
							ProjectileID: projectile.ID,
							CreatedAt:    now,
						},
					})
					break
				}
			}

			if hitTargetID != "" || projectile.Distance >= projectile.MaxDistance {
				delete(room.projectiles, projectileID)
				queued = append(queued, queuedEvent{
					recipients: recipients,
					event: ServerEvent{
						Type:         "projectile_removed",
						Room:         room.id,
						ProjectileID: projectileID,
						CreatedAt:    now,
					},
				})
			}
		}
	}
	h.mu.Unlock()

	for _, item := range queued {
		h.broadcast(item.recipients, item.event)
	}
}

func (h *Hub) Leave(playerID string) {
	now := time.Now().UTC()

	h.mu.Lock()
	roomID, ok := h.players[playerID]
	if !ok {
		h.mu.Unlock()
		return
	}
	room := h.rooms[roomID]
	delete(room.players, playerID)
	delete(room.peers, playerID)
	delete(h.players, playerID)
	recipients := roomPeersExcept(room, playerID)
	h.mu.Unlock()

	event := ServerEvent{
		Type:      "player_left",
		Room:      roomID,
		PlayerID:  playerID,
		CreatedAt: now,
	}
	h.broadcast(recipients, event)
	h.publish("battle.events.world.player_left", event)
}

func (h *Hub) State(roomID string) (RoomState, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room := h.rooms[roomID]
	if room == nil {
		return RoomState{}, ErrInvalidRoom
	}
	return roomSnapshot(room), nil
}

func (h *Hub) broadcast(peers map[string]Peer, event ServerEvent) {
	for _, peer := range peers {
		peer.Send(event)
	}
}

func (h *Hub) publish(subject string, event ServerEvent) {
	if h.publisher == nil {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		h.logger.Warn("failed to encode world event", "error", err)
		return
	}
	if err := h.publisher.PublishBattleEvent(subject, payload); err != nil {
		h.logger.Warn("failed to publish world event", "subject", subject, "error", err)
	}
}

func maxInt32(a int32, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func minInt32(a int32, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func absFloat64(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
