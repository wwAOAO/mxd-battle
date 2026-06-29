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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"mxd-battle/internal/world"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type Handler struct {
	hub           *Hub
	logger        *slog.Logger
	roleProvider  RoleProvider
	worldMapsFile string
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

func NewHandler(hub *Hub, logger *slog.Logger, roleProvider RoleProvider, worldMapsFile ...string) *Handler {
	handler := &Handler{hub: hub, logger: logger, roleProvider: roleProvider}
	if len(worldMapsFile) > 0 {
		handler.worldMapsFile = worldMapsFile[0]
	}
	return handler
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/job-stats", h.jobStats)
	mux.HandleFunc("/equipment-stats", h.equipmentStats)
	mux.HandleFunc("/monster-stats", h.monsterStats)
	mux.HandleFunc("/config-files", h.configFiles)
	mux.HandleFunc("/config-templates", h.configTemplates)
	mux.HandleFunc("/config-files/", h.configFile)
	mux.HandleFunc("/map-files", h.mapFiles)
	mux.HandleFunc("/map-files/", h.mapFile)
	mux.HandleFunc("/rooms", h.rooms)
	mux.HandleFunc("/rooms/", h.roomState)
	mux.HandleFunc("/ws", h.websocket)
	mux.Handle("/", noCacheFileServer(http.Dir("web")))
}

func noCacheFileServer(root http.FileSystem) http.Handler {
	fileServer := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})
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

func (h *Handler) monsterStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.hub.MonsterStatConfigs())
}

type mapFileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (h *Handler) mapFiles(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/map-files" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "map file not found"})
		return
	}

	files := make([]mapFileInfo, 0)
	root := filepath.Join("config", "maps")
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, mapFileInfo{
			Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			Path: filepath.ToSlash(rel),
		})
		return nil
	}); err != nil {
		h.logger.Warn("failed to list map files", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list map files"})
		return
	}
	slices.SortFunc(files, func(a mapFileInfo, b mapFileInfo) int {
		return strings.Compare(a.Path, b.Path)
	})
	writeJSON(w, http.StatusOK, map[string][]mapFileInfo{"maps": files})
}

func (h *Handler) mapFile(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/map-files/")
	if rel == "" || strings.Contains(rel, "\\") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "map file path required"})
		return
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) || filepath.Ext(clean) != ".json" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid map file path"})
		return
	}

	path := filepath.Join("config", "maps", clean)
	if r.Method == http.MethodPut {
		h.saveMapFile(w, r, path)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "map file not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

type saveMapFileResponse struct {
	Status        string   `json:"status"`
	ReloadedRooms []string `json:"reloadedRooms,omitempty"`
	KickedPlayers int      `json:"kickedPlayers"`
}

func (h *Handler) saveMapFile(w http.ResponseWriter, r *http.Request, path string) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
		return
	}
	if len(payload) == 2*1024*1024 {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "map file is too large"})
		return
	}

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid map json: " + err.Error()})
		return
	}

	formatted, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to format map json"})
		return
	}
	formatted = append(formatted, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		h.logger.Warn("failed to create map directory", "path", path, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create map directory"})
		return
	}
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		h.logger.Warn("failed to save map file", "path", path, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save map file"})
		return
	}

	response := saveMapFileResponse{Status: "saved"}
	if h.worldMapsFile != "" {
		roomIDs, err := world.RoomIDsForMapPath(h.worldMapsFile, path)
		if err != nil {
			h.logger.Warn("failed to find rooms for saved map file", "path", path, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "saved map, but failed to find rooms to reload"})
			return
		}
		if len(roomIDs) > 0 {
			maps, err := world.LoadMaps(h.worldMapsFile)
			if err != nil {
				h.logger.Warn("failed to reload world maps after save", "path", h.worldMapsFile, "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "saved map, but failed to reload world maps"})
				return
			}
			reloadedRooms, kickedPlayers, err := h.hub.ReloadRooms(maps, roomIDs)
			if err != nil {
				h.logger.Warn("failed to apply reloaded rooms", "rooms", roomIDs, "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "saved map, but failed to apply reloaded rooms"})
				return
			}
			response.ReloadedRooms = reloadedRooms
			response.KickedPlayers = kickedPlayers
		}
	}

	writeJSON(w, http.StatusOK, response)
}

type configFileInfo struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Path     string `json:"path"`
}

var editableConfigRoots = map[string]string{
	"equipment":  filepath.Join("config", "equipment"),
	"job_stats":  filepath.Join("config", "job_stats"),
	"job_skills": filepath.Join("config", "job_skills"),
}

func (h *Handler) configTemplates(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/config-templates" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "config template not found"})
		return
	}

	templates := make(map[string]any)
	for category := range editableConfigRoots {
		path := filepath.Join("config", "templates", category+".json")
		payload, err := os.ReadFile(path)
		if err != nil {
			h.logger.Warn("failed to read config template", "category", category, "path", path, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read config templates"})
			return
		}
		var value any
		if err := json.Unmarshal(payload, &value); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid config template " + category})
			return
		}
		templates[category] = value
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
}
func (h *Handler) configFiles(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/config-files" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "config file not found"})
		return
	}

	files := make([]configFileInfo, 0)
	for category, root := range editableConfigRoots {
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".json" {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, configFileInfo{
				Category: category,
				Name:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
				Path:     filepath.ToSlash(rel),
			})
			return nil
		}); err != nil {
			h.logger.Warn("failed to list config files", "category", category, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list config files"})
			return
		}
	}
	slices.SortFunc(files, func(a configFileInfo, b configFileInfo) int {
		if a.Category != b.Category {
			return strings.Compare(a.Category, b.Category)
		}
		return strings.Compare(a.Path, b.Path)
	})
	writeJSON(w, http.StatusOK, map[string][]configFileInfo{"files": files})
}

func (h *Handler) configFile(w http.ResponseWriter, r *http.Request) {
	category, clean, ok := parseEditableConfigPath(strings.TrimPrefix(r.URL.Path, "/config-files/"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid config file path"})
		return
	}
	root := editableConfigRoots[category]
	path := filepath.Join(root, clean)
	if r.Method == http.MethodPut {
		h.saveConfigFile(w, r, path)
		return
	}
	if r.Method == http.MethodDelete {
		h.deleteConfigFile(w, path)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "config file not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func parseEditableConfigPath(value string) (string, string, bool) {
	category, rel, ok := strings.Cut(value, "/")
	if !ok || category == "" || rel == "" || strings.Contains(rel, "\\") {
		return "", "", false
	}
	if _, ok := editableConfigRoots[category]; !ok {
		return "", "", false
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) || filepath.Ext(clean) != ".json" {
		return "", "", false
	}
	return category, clean, true
}

func (h *Handler) saveConfigFile(w http.ResponseWriter, r *http.Request, path string) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
		return
	}
	if len(payload) == 2*1024*1024 {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "config file is too large"})
		return
	}

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid config json: " + err.Error()})
		return
	}
	formatted, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to format config json"})
		return
	}
	formatted = append(formatted, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		h.logger.Warn("failed to create config directory", "path", path, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create config directory"})
		return
	}
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		h.logger.Warn("failed to save config file", "path", path, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config file"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *Handler) deleteConfigFile(w http.ResponseWriter, path string) {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "config file not found"})
			return
		}
		h.logger.Warn("failed to delete config file", "path", path, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete config file"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
			if msg.Type == "pickup" {
				_, _, _ = hub.PickupLoot(c.playerID)
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
