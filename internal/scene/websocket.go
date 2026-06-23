package scene

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type Handler struct {
	hub          *Hub
	logger       *slog.Logger
	roleProvider RoleProvider
}

type RoleProvider interface {
	GetAccountRole(ctx context.Context, accountID string, roleID string) (RoleSnapshot, error)
}

type RoleSnapshot struct {
	ID        string
	AccountID string
	Nickname  string
	Level     int32
	Exp       string
	JobCode   string
	Gender    string
	Stat      PlayerStatBundle
	MapID     int32
	X         float64
	Y         float64
}

type moveMessage struct {
	Type             string            `json:"type"`
	X                float64           `json:"x"`
	Y                float64           `json:"y"`
	InputX           float64           `json:"inputX"`
	InputY           float64           `json:"inputY"`
	Stat             PlayerStat        `json:"stat"`
	SkillID          string            `json:"skillId"`
	EquipmentIDs     []string          `json:"equipmentIds"`
	EquipmentsBySlot map[string]string `json:"equipmentsBySlot"`
}

type websocketClient struct {
	conn     net.Conn
	reader   *bufio.Reader
	logger   *slog.Logger
	playerID string
	roomID   string
	send     chan ServerEvent
	writeMu  sync.Mutex
}

func NewHandler(hub *Hub, logger *slog.Logger, roleProvider RoleProvider) *Handler {
	return &Handler{hub: hub, logger: logger, roleProvider: roleProvider}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/job-stats", h.jobStats)
	mux.HandleFunc("/equipment-stats", h.equipmentStats)
	mux.HandleFunc("/rooms", h.rooms)
	mux.HandleFunc("/rooms/", h.roomState)
	mux.HandleFunc("/ws", h.websocket)
	mux.Handle("/", http.FileServer(http.Dir("web")))
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) rooms(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{"rooms": h.hub.Rooms()})
}

func (h *Handler) jobStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.hub.JobStatConfigs())
}

func (h *Handler) equipmentStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.hub.EquipmentConfigs())
}

func (h *Handler) roomState(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimPrefix(r.URL.Path, "/rooms/")
	if roomID == "" || strings.Contains(roomID, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "room not found"})
		return
	}

	state, err := h.hub.State(roomID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *Handler) websocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	playerID := r.URL.Query().Get("player")
	accountID := r.URL.Query().Get("account")
	roleID := r.URL.Query().Get("role")
	jobCode := r.URL.Query().Get("job")
	gender := r.URL.Query().Get("gender")
	name := r.URL.Query().Get("name")
	if roomID == "" {
		roomID = h.hub.DefaultRoom()
	}
	if playerID == "" {
		playerID = roleID
	}

	role, err := h.resolveRole(r.Context(), accountID, roleID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	if role.ID != "" {
		playerID = role.ID
		if name == "" {
			name = role.Nickname
		}
		if jobCode == "" {
			jobCode = role.JobCode
		}
		if gender == "" {
			gender = role.Gender
		}
	}

	conn, reader, err := upgrade(w, r)
	if err != nil {
		h.logger.Warn("websocket upgrade failed", "error", err)
		return
	}

	client := &websocketClient{
		conn:     conn,
		reader:   reader,
		logger:   h.logger,
		playerID: playerID,
		roomID:   roomID,
		send:     make(chan ServerEvent, 32),
	}
	defer conn.Close()

	state, err := h.hub.Join(roomID, Player{
		ID:      playerID,
		Name:    name,
		Level:   role.Level,
		Exp:     role.Exp,
		JobCode: jobCode,
		Gender:  gender,
		Stat:    role.Stat,
		X:       role.X,
		Y:       role.Y,
		MapID:   fmt.Sprint(role.MapID),
	}, client)
	if err != nil {
		client.writeEvent(ServerEvent{
			Type:      "error",
			Message:   err.Error(),
			CreatedAt: time.Now().UTC(),
		})
		return
	}
	defer h.hub.Leave(playerID)

	client.writeEvent(ServerEvent{
		Type:      "snapshot",
		Room:      roomID,
		State:     &state,
		CreatedAt: time.Now().UTC(),
	})

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go client.writeLoop(ctx)
	client.readLoop(h.hub)
}

func (h *Handler) resolveRole(ctx context.Context, accountID string, roleID string) (RoleSnapshot, error) {
	if accountID == "" && roleID == "" {
		return RoleSnapshot{}, nil
	}
	if accountID == "" || roleID == "" {
		return RoleSnapshot{}, errors.New("account and role are required together")
	}
	if h.roleProvider == nil {
		return RoleSnapshot{}, errors.New("backend grpc is unavailable")
	}
	return h.roleProvider.GetAccountRole(ctx, accountID, roleID)
}

func (c *websocketClient) Send(event ServerEvent) {
	select {
	case c.send <- event:
	default:
		c.logger.Warn("dropping world event for slow websocket client", "player_id", c.playerID)
	}
}

func (c *websocketClient) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-c.send:
			c.writeEvent(event)
		}
	}
}

func (c *websocketClient) readLoop(hub *Hub) {
	for {
		payload, opcode, err := readFrame(c.reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.logger.Warn("failed to read websocket frame", "player_id", c.playerID, "error", err)
			}
			return
		}

		switch opcode {
		case 0x1:
			var msg moveMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				c.writeEvent(ServerEvent{
					Type:      "error",
					Message:   "invalid json message",
					CreatedAt: time.Now().UTC(),
				})
				continue
			}
			if msg.Type == "jump" {
				player, ok := hub.Jump(c.playerID)
				if ok {
					c.writeEvent(ServerEvent{
						Type:      "player_moved",
						Room:      player.Room,
						Player:    &player,
						CreatedAt: time.Now().UTC(),
					})
				}
				continue
			}
			if msg.Type == "drop" {
				player, ok := hub.Drop(c.playerID)
				if ok {
					c.writeEvent(ServerEvent{
						Type:      "player_moved",
						Room:      player.Room,
						Player:    &player,
						CreatedAt: time.Now().UTC(),
					})
				}
				continue
			}
			if msg.Type == "portal" {
				player, ok := hub.UsePortal(c.playerID)
				if ok {
					c.roomID = player.Room
				}
				continue
			}
			if msg.Type == "input" {
				_, _ = hub.SetInput(c.playerID, msg.InputX, msg.InputY)
				continue
			}
			if msg.Type == "stat" {
				player, ok := hub.SetPrimaryStat(c.playerID, msg.Stat)
				if ok {
					c.writeEvent(ServerEvent{
						Type:      "player_stat_updated",
						Room:      player.Room,
						Player:    &player,
						CreatedAt: time.Now().UTC(),
					})
				}
				continue
			}
			if msg.Type == "attack" {
				_, _, _ = hub.NormalAttack(c.playerID)
				continue
			}
			if msg.Type == "equipment" {
				equipmentIDs := msg.EquipmentIDs
				if len(msg.EquipmentsBySlot) > 0 {
					equipmentIDs = orderedEquipmentIDsFromSlots(msg.EquipmentsBySlot)
				}
				player, selection, ok := hub.SetEquipment(c.playerID, equipmentIDs)
				if ok {
					c.writeEvent(ServerEvent{
						Type:        "player_stat_updated",
						Room:        player.Room,
						Player:      &player,
						EquipResult: &selection,
						CreatedAt:   time.Now().UTC(),
					})
				}
				continue
			}
			if msg.Type == "skill" {
				_, _, _ = hub.CastSkill(c.playerID, msg.SkillID)
				continue
			}
			if msg.Type != "move" {
				continue
			}
			previousRoomID := c.roomID
			player, ok := hub.Move(c.playerID, msg.X, msg.Y)
			if ok {
				c.roomID = player.Room
				if player.Room != previousRoomID {
					continue
				}
				c.writeEvent(ServerEvent{
					Type:      "player_moved",
					Room:      player.Room,
					Player:    &player,
					CreatedAt: time.Now().UTC(),
				})
			}
		case 0x8:
			_ = c.writeFrame(0x8, nil)
			return
		case 0x9:
			_ = c.writeFrame(0xA, payload)
		}
	}
}

func (c *websocketClient) writeEvent(event ServerEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		c.logger.Warn("failed to encode websocket event", "error", err)
		return
	}
	if err := c.writeFrame(0x1, payload); err != nil {
		c.logger.Warn("failed to write websocket event", "player_id", c.playerID, "error", err)
	}
}

func (c *websocketClient) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	header := []byte{0x80 | opcode}
	switch {
	case len(payload) < 126:
		header = append(header, byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, 127)
		length := make([]byte, 8)
		binary.BigEndian.PutUint64(length, uint64(len(payload)))
		header = append(header, length...)
	}

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	_, err := c.conn.Write(payload)
	return err
}

func upgrade(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.Reader, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "upgrade header required", http.StatusBadRequest)
		return nil, nil, errors.New("upgrade header required")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "sec-websocket-key required", http.StatusBadRequest)
		return nil, nil, errors.New("sec-websocket-key required")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket unsupported", http.StatusInternalServerError)
		return nil, nil, errors.New("http hijacker unsupported")
	}

	conn, buffered, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}

	accept := websocketAccept(key)
	response := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n\r\n",
		accept,
	)
	if _, err := conn.Write([]byte(response)); err != nil {
		conn.Close()
		return nil, nil, err
	}

	return conn, buffered.Reader, nil
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func readFrame(reader *bufio.Reader) ([]byte, byte, error) {
	header, err := reader.ReadByte()
	if err != nil {
		return nil, 0, err
	}
	lengthByte, err := reader.ReadByte()
	if err != nil {
		return nil, 0, err
	}

	opcode := header & 0x0F
	masked := lengthByte&0x80 != 0
	length := uint64(lengthByte & 0x7F)
	switch length {
	case 126:
		var extended [2]byte
		if _, err := io.ReadFull(reader, extended[:]); err != nil {
			return nil, 0, err
		}
		length = uint64(binary.BigEndian.Uint16(extended[:]))
	case 127:
		var extended [8]byte
		if _, err := io.ReadFull(reader, extended[:]); err != nil {
			return nil, 0, err
		}
		length = binary.BigEndian.Uint64(extended[:])
	}
	if length > 64*1024 {
		return nil, 0, errors.New("websocket frame too large")
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(reader, mask[:]); err != nil {
			return nil, 0, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, 0, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return payload, opcode, nil
}

func orderedEquipmentIDsFromSlots(equipmentsBySlot map[string]string) []string {
	order := []string{"weapon_main", "weapon_sub", "ring1", "ring2", "armor", "shoes", "accessory", "misc"}
	ids := make([]string, 0, len(equipmentsBySlot))
	seen := make(map[string]struct{})
	for _, slot := range order {
		if id := strings.TrimSpace(equipmentsBySlot[slot]); id != "" {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	for _, id := range equipmentsBySlot {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
