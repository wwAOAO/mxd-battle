package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mxd-battle/internal/backend"
	"mxd-battle/internal/combat"
	"mxd-battle/internal/config"
	"mxd-battle/internal/messaging"
	"mxd-battle/internal/scene"
	"mxd-battle/internal/world"
)

type sceneRoleProvider struct {
	client *backend.Client
}

func (p sceneRoleProvider) GetAccountRole(ctx context.Context, accountID string, roleID string) (scene.RoleSnapshot, error) {
	role, err := p.client.GetAccountRole(ctx, accountID, roleID)
	if err != nil {
		return scene.RoleSnapshot{}, err
	}

	return scene.RoleSnapshot{
		ID:        role.ID,
		AccountID: role.AccountID,
		Nickname:  role.Nickname,
		Level:     role.Level,
		Exp:       role.Exp,
		JobCode:   role.JobCode,
		Gender:    "",
		Stat: scene.PlayerStatBundle{
			Base: scene.PlayerStat{
				Strength:     role.Strength,
				Intelligence: role.Intelligence,
				Agility:      role.Agility,
				Luck:         role.Luck,
				HP:           role.HP,
				MP:           role.MP,
				HPMax:        role.HPMax,
				MPMax:        role.MPMax,
			},
		},
		MapID: role.MapID,
		X:     role.X,
		Y:     role.Y,
	}, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()
	worldMaps, err := world.LoadMaps(cfg.WorldMapsFile)
	if err != nil {
		logger.Error("failed to load world maps config", "path", cfg.WorldMapsFile, "error", err)
		os.Exit(1)
	}
	jobStats, err := combat.LoadJobStatConfigs(cfg.JobStatsFile)
	if err != nil {
		logger.Error("failed to load job stat config", "path", cfg.JobStatsFile, "error", err)
		os.Exit(1)
	}
	skillStats, err := combat.LoadSkillConfigs(cfg.SkillStatsFile)
	if err != nil {
		logger.Error("failed to load skill config", "path", cfg.SkillStatsFile, "error", err)
		os.Exit(1)
	}
	equipmentStats, err := combat.LoadEquipmentConfigs(cfg.EquipmentStatsFile)
	if err != nil {
		logger.Error("failed to load equipment config", "path", cfg.EquipmentStatsFile, "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backendClient, err := backend.NewClient(cfg.BackendGRPC)
	if err != nil {
		logger.Warn("backend grpc unavailable; scene joins will use query parameters only", "target", cfg.BackendGRPC, "error", err)
	} else {
		defer backendClient.Close()
	}

	var client *messaging.JetStreamClient
	client, err = messaging.NewJetStreamClient(cfg, logger)
	if err != nil {
		logger.Warn("nats unavailable; world sync will run without jetstream persistence", "error", err)
	} else {
		defer client.Close()

		if err := client.EnsureBattleStream(); err != nil {
			logger.Warn("jetstream unavailable; world sync will run without event persistence", "error", err)
			client.Close()
			client = nil
		} else if err := client.SubscribeBattleEvents(ctx); err != nil {
			logger.Warn("battle event subscriber unavailable; world sync will run without event persistence", "error", err)
			client.Close()
			client = nil
		}
	}

	var publisher scene.EventPublisher
	if client != nil {
		publisher = client
	}

	hub, err := scene.NewHubWithJobsAndEquipment(logger, publisher, worldMaps, jobStats, equipmentStats, skillStats)
	if err != nil {
		logger.Error("failed to initialize scene hub", "error", err)
		os.Exit(1)
	}
	go hub.StartPhysics(ctx)

	mux := http.NewServeMux()
	var roleProvider scene.RoleProvider
	if backendClient != nil {
		roleProvider = sceneRoleProvider{client: backendClient}
	}
	scene.NewHandler(hub, logger, roleProvider).Register(mux)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("mxd battle realtime scene listening", "http_addr", cfg.HTTPAddr, "rooms", hub.Rooms())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err)
			stop()
		}
	}()

	logger.Info("mxd battle service started",
		"service", cfg.ServiceName,
		"nats_url", cfg.NATSURL,
		"stream", cfg.BattleStream,
		"http_addr", cfg.HTTPAddr,
		"backend_grpc", cfg.BackendGRPC,
	)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http server did not stop cleanly", "error", err)
	}

	if client != nil {
		if err := client.Drain(shutdownCtx); err != nil {
			logger.Warn("nats drain did not finish cleanly", "error", err)
		}
	}

	logger.Info("mxd battle service stopped")
}
