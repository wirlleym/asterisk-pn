package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// asterisk-push-notify — CLI de push (sem daemon).
//
// Uso:
//   asterisk-push-notify <ramal> [caller]                 # dispara o push do ramal
//   asterisk-push-notify --register <ramal> <token> [provider] [deviceId]
//   asterisk-push-notify --list
//
// Config por variavel de ambiente (ver INSTALL.md). O dialplan chama o modo
// "notify" via System(); o modo "--register" e usado para cadastrar o token.

func main() {
	cfg := loadConfig()
	args := os.Args[1:]

	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "--help", "-h", "help":
		usage()
	case "--list", "list":
		if err := list(cfg); err != nil {
			fatal(err)
		}
	case "--register", "register":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "uso: asterisk-push-notify --register <ramal> <token> [provider] [deviceId]")
			os.Exit(2)
		}
		if err := register(cfg, args); err != nil {
			fatal(err)
		}
		fmt.Println("ok")
	default:
		// modo notify: nunca falha a chamada por causa do push.
		if err := notify(cfg, args[0], callerOf(args)); err != nil {
			fmt.Fprintln(os.Stderr, "asterisk-push-notify:", err)
		}
		os.Exit(0)
	}
}

func callerOf(args []string) string {
	if len(args) > 1 {
		return args[1]
	}
	return ""
}

func usage() {
	fmt.Println(`asterisk-push-notify — push VoIP (VoIPforall)

  asterisk-push-notify <ramal> [caller]                    dispara o push
  asterisk-push-notify --register <ramal> <token> [provider] [deviceId]
  asterisk-push-notify --list                              lista os tokens
  asterisk-push-notify --help

Variaveis: PUSH_DATA_FILE, APNS_KEY_PATH, APNS_KEY_ID, APNS_TEAM_ID,
APNS_TOPIC, APNS_ENVIRONMENT, FCM_SERVICE_ACCOUNT_JSON`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "erro:", err)
	os.Exit(1)
}

func list(cfg Config) error {
	store, err := NewStore(cfg.DataFile)
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(store.devices, "", "  ")
	fmt.Println(string(data))
	return nil
}

func register(cfg Config, args []string) error {
	ramal := strings.TrimSpace(args[1])
	token := strings.TrimSpace(args[2])
	if ramal == "" || token == "" {
		return fmt.Errorf("ramal e token sao obrigatorios")
	}
	provider := "apns_voip"
	if len(args) > 3 && strings.TrimSpace(args[3]) != "" {
		provider = strings.TrimSpace(args[3])
	}
	platform := "ios"
	if provider == "fcm" {
		platform = "android"
	}
	deviceID := ramal + "-1"
	if len(args) > 4 && strings.TrimSpace(args[4]) != "" {
		deviceID = strings.TrimSpace(args[4])
	}
	store, err := NewStore(cfg.DataFile)
	if err != nil {
		return err
	}
	return store.Upsert(DeviceRecord{
		Extension: ramal,
		Platform:  platform,
		Provider:  provider,
		Token:     token,
		DeviceID:  deviceID,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func notify(cfg Config, ramal, caller string) error {
	ramal = strings.TrimSpace(ramal)
	if ramal == "" {
		return fmt.Errorf("ramal vazio")
	}
	store, err := NewStore(cfg.DataFile)
	if err != nil {
		return err
	}
	devices := store.FindByExtension(ramal)
	withToken := make([]DeviceRecord, 0, len(devices))
	for _, d := range devices {
		if strings.TrimSpace(d.Token) != "" {
			withToken = append(withToken, d)
		}
	}
	if len(withToken) == 0 {
		return fmt.Errorf("sem token para o ramal %s", ramal)
	}
	if caller == "" {
		caller = ramal
	}
	callID := newID()
	ttl := 30

	lastReason := "provider_unavailable"
	for _, d := range withToken {
		var delivered bool
		var reason string
		switch d.Provider {
		case "apns_voip":
			delivered, reason = newAPNSSender(cfg).Send(d, callID, caller, ttl)
		case "fcm":
			delivered, reason = newFCMSender(cfg).Send(d, callID, caller, ttl)
		default:
			reason = "provider_unavailable"
		}
		if delivered {
			fmt.Printf("delivered=true provider=%s ramal=%s\n", d.Provider, ramal)
			return nil
		}
		lastReason = reason
	}
	return fmt.Errorf("nao entregue: %s", lastReason)
}

func newID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
