package config

import "os"

type Config struct {
	ServiceName        string
	HTTPAddr           string
	BackendGRPC        string
	NATSURL            string
	BattleStream       string
	BattleSubject      string
	WorldMapsFile      string
	JobStatsFile       string
	SkillStatsFile     string
	EquipmentStatsFile string
}

func Load() Config {
	return Config{
		ServiceName:        getEnv("SERVICE_NAME", "mxd-battle"),
		HTTPAddr:           getEnv("HTTP_ADDR", ":8080"),
		BackendGRPC:        getEnv("BACKEND_GRPC_ADDR", "127.0.0.1:50051"),
		NATSURL:            getEnv("NATS_URL", "nats://127.0.0.1:4222"),
		BattleStream:       getEnv("BATTLE_STREAM", "MXD_BATTLE"),
		BattleSubject:      getEnv("BATTLE_SUBJECT", "battle.events.>"),
		WorldMapsFile:      getEnv("WORLD_MAPS_FILE", "config/world_maps.json"),
		JobStatsFile:       getEnv("JOB_STATS_FILE", "config/job_stats.json"),
		SkillStatsFile:     getEnv("SKILL_STATS_FILE", "config/skill_stats.json"),
		EquipmentStatsFile: getEnv("EQUIPMENT_STATS_FILE", "config/equipment"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
