package main

import (
	"os"
	"strings"
)

// Config do CLI. Os valores vem de variaveis de ambiente e, se ausentes, de um
// arquivo KEY=VALUE (default: /etc/asterisk-push-notify/push.env) — util quando o CLI e
// chamado pelo dialplan, que nao propaga o ambiente do shell.
type Config struct {
	DataFile          string
	APNSKeyPath       string
	APNSKeyID         string
	APNSTeamID        string
	APNSTopic         string
	APNSEnvironment   string
	FCMServiceAccount string
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// loadEnvFile le um arquivo KEY=VALUE e define no ambiente somente o que ainda
// nao estiver definido (variavel de ambiente tem prioridade).
func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func loadConfig() Config {
	loadEnvFile(env("PUSH_ENV_FILE", "/etc/asterisk-push-notify/push.env"))
	return Config{
		DataFile:          env("PUSH_DATA_FILE", "/etc/asterisk-push-notify/devices.json"),
		APNSKeyPath:       env("APNS_KEY_PATH", ""),
		APNSKeyID:         env("APNS_KEY_ID", ""),
		APNSTeamID:        env("APNS_TEAM_ID", ""),
		APNSTopic:         env("APNS_TOPIC", ""),
		APNSEnvironment:   env("APNS_ENVIRONMENT", "production"),
		FCMServiceAccount: env("FCM_SERVICE_ACCOUNT_JSON", ""),
	}
}
