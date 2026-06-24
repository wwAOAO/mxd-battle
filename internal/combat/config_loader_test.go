package combat

import "testing"

func TestLoadJobStatConfigsFromDirectory(t *testing.T) {
	configs, err := LoadJobStatConfigs("../../config/job_stats")
	if err != nil {
		t.Fatalf("load job stats: %v", err)
	}
	if _, ok := configs["beginner"]; !ok {
		t.Fatalf("expected beginner job config, got %+v", configs)
	}
}

func TestLoadSkillConfigsFromDirectory(t *testing.T) {
	configs, err := LoadSkillConfigs("../../config/job_skills")
	if err != nil {
		t.Fatalf("load skill configs: %v", err)
	}
	if config, ok := configs["magic_missile"]; !ok || config.ID != "magic_missile" {
		t.Fatalf("expected magic_missile skill config, got %+v", configs)
	}
}

func TestLoadMonsterStatConfigsFromDirectory(t *testing.T) {
	configs, err := LoadMonsterStatConfigs("../../config/monster_stats")
	if err != nil {
		t.Fatalf("load monster stats: %v", err)
	}
	if config, ok := configs["green_slime"]; !ok || config.ID != "green_slime" {
		t.Fatalf("expected green_slime monster config, got %+v", configs)
	}
	if config, ok := configs["orange_mushroom"]; !ok || config.ID != "orange_mushroom" {
		t.Fatalf("expected orange_mushroom monster config, got %+v", configs)
	}
}
