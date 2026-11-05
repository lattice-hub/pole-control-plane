package config

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pole-io/pole-server/apis/access_control/auth"
	"github.com/pole-io/pole-server/pkg/systemconfig"
)

func TestNewSystemSettingsProviderReportsEffectiveDefaultsAndRedactsSecrets(t *testing.T) {
	cfg := &Config{
		Bootstrap: defaultBootstrap(),
		Auth: auth.Config{
			User: &auth.UserConfig{Option: map[string]interface{}{"salt": "private-salt"}},
		},
	}
	cfg.SystemConfigSources = systemconfig.SourceIndex{
		"bootstrap.mode":                      {Kind: systemconfig.SourceStaticFile},
		"auth.user.option.salt":               {Kind: systemconfig.SourceEnvironment, Reference: "env:AUTH_SALT"},
		"naming.healthcheck.minCheckInterval": {Kind: systemconfig.SourceStaticFile},
		"workloadCredential.ttl":              {Kind: systemconfig.SourceStaticFile},
	}

	provider, err := NewSystemSettingsProvider(cfg)
	if err != nil {
		t.Fatalf("NewSystemSettingsProvider() error = %v", err)
	}
	snapshot, err := provider.Effective(context.Background(), systemconfig.Scope{Component: systemconfig.ComponentServer})
	if err != nil {
		t.Fatalf("Effective() error = %v", err)
	}

	settings := settingsByKey(snapshot.Settings)
	if got := settings["server.bootstrap.mode"].Source.Kind; got != systemconfig.SourceStaticFile {
		t.Fatalf("mode source = %q", got)
	}
	if got := settings["server.naming.healthcheck.min_check_interval"].Value; got != "1s" {
		t.Fatalf("healthcheck min interval = %#v, want 1s", got)
	}
	if got := settings["server.naming.healthcheck.min_check_interval"].Source.Kind; got != systemconfig.SourceCompiledDefault {
		t.Fatalf("normalized healthcheck source = %q", got)
	}
	if got := settings["server.workload_credential.ttl"].Value; got != (5 * time.Minute).String() {
		t.Fatalf("credential ttl = %#v, want 5m0s", got)
	}
	if got := settings["server.workload_credential.ttl"].Source.Kind; got != systemconfig.SourceCompiledDefault {
		t.Fatalf("normalized workload TTL source = %q", got)
	}
	salt := settings["server.auth.user.salt"]
	if !salt.Redacted || salt.Value != nil || salt.Source.Reference != "env:AUTH_SALT" {
		t.Fatalf("salt setting = %#v", salt)
	}
	body, _ := json.Marshal(snapshot)
	if strings.Contains(string(body), "private-salt") {
		t.Fatalf("snapshot leaked auth salt: %s", body)
	}
}

func TestServerSystemSettingsHaveExplicitFieldEditPolicies(t *testing.T) {
	provider, err := NewSystemSettingsProvider(&Config{})
	if err != nil {
		t.Fatalf("NewSystemSettingsProvider() error = %v", err)
	}
	definitions, err := provider.Describe(context.Background(), systemconfig.Scope{Component: systemconfig.ComponentServer})
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if len(definitions) != 30 {
		t.Fatalf("definition count = %d, want 30", len(definitions))
	}
	editable := 0
	for _, definition := range definitions {
		if definition.Description == "" || definition.EditReason == "" {
			t.Fatalf("setting %s has no explicit edit policy: %#v", definition.Key, definition)
		}
		if definition.Editable {
			editable++
		}
	}
	if editable != 22 {
		t.Fatalf("editable count = %d, want 22", editable)
	}
}

func settingsByKey(settings []systemconfig.EffectiveSetting) map[string]systemconfig.EffectiveSetting {
	result := make(map[string]systemconfig.EffectiveSetting, len(settings))
	for _, setting := range settings {
		result[setting.Key] = setting
	}
	return result
}
