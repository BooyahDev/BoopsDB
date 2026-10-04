package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoUpdateDefaultsAndUnknownKeysSurviveRegistration(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         bool
	}{
		{"default", `{"id":"old","future":{"nested":[1,"two"]}}`, true},
		{"disabled", `{"id":"old","auto_update":false,"future":{"nested":[1,"two"]}}`, false},
		{"enabled", `{"id":"old","auto_update":true,"future":{"nested":[1,"two"]}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := configPath
			configPath = filepath.Join(t.TempDir(), "nested", "config.json")
			defer func() { configPath = original }()
			if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configPath, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AutoUpdateEnabled() != tc.want {
				t.Fatalf("auto_update=%v", cfg.AutoUpdateEnabled())
			}
			if err := SaveConfig("new-id"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			var values map[string]json.RawMessage
			if err = json.Unmarshal(data, &values); err != nil {
				t.Fatal(err)
			}
			if string(values["id"]) != `"new-id"` || string(values["future"]) != `{"nested":[1,"two"]}` {
				t.Fatalf("keys changed: %s", data)
			}
			cfg, err = LoadConfig()
			if err != nil || cfg.AutoUpdateEnabled() != tc.want {
				t.Fatalf("after registration cfg=%v err=%v", cfg, err)
			}
			info, err := os.Stat(configPath)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("config mode changed: %v %v", info, err)
			}
		})
	}
}

func TestSaveConfigDoesNotReplaceBrokenConfig(t *testing.T) {
	original := configPath
	configPath = filepath.Join(t.TempDir(), "config.json")
	defer func() { configPath = original }()
	before := []byte(`{"id":"old",broken`)
	if err := os.WriteFile(configPath, before, 0644); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig("new-id"); err == nil {
		t.Fatal("broken config replaced")
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("broken config bytes changed")
	}
}

func TestSaveConfigCreatesParentAndDefaultsAutoUpdate(t *testing.T) {
	original := configPath
	configPath = filepath.Join(t.TempDir(), "nested", "config.json")
	defer func() { configPath = original }()
	if err := SaveConfig("new-id"); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil || cfg.ID != "new-id" || !cfg.AutoUpdateEnabled() {
		t.Fatalf("cfg=%v err=%v", cfg, err)
	}
}
