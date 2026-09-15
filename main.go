package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// asterisk-push-notify — acordador de softphone (CLI, sem daemon).
//
// Responsabilidade: checar se o app está vivo (SIP OPTIONS) e, se estiver
// "dormindo" (morto), enviar o push para acordá-lo. O dialplan então disca.
//
// Uso:
//   asterisk-push-notify <ramal> [contacts] [caller]   # checa vivo; morto → push
//   asterisk-push-notify --register <ramal> <token> [provider] [deviceId]
//   asterisk-push-notify --list
//
// Codigo de saida (o dialplan usa ${SYSTEMSTATUS} para decidir):
//   0 = app vivo (discar direto, sem push)
//   1 = app morto (push enviado; o dialplan espera o app re-registrar e disca)
//
// Config por variavel de ambiente (ver INSTALL.md). O dialplan passa os
// contatos via ${PJSIP_DIAL_CONTACTS(...)} para o OPTIONS.

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
		// modo notify: <ramal> [contacts] [caller]
		ramal := args[0]
		contacts := argOr(args, 1, "")
		caller := argOr(args, 2, "")

		if hasAlwaysOnline(contacts) {
			fmt.Fprintln(os.Stderr, "always_online=true (desktop/web presente; sem push)")
			os.Exit(0)
		}
		if checkAlive(contacts) {
			fmt.Fprintln(os.Stderr, "alive=true (sem push)")
			os.Exit(0)
		}
		// app morto (ou sem contato) → push
		if err := sendPush(cfg, ramal, caller); err != nil {
			fmt.Fprintln(os.Stderr, "dead=true push=failed:", err)
		} else {
			fmt.Fprintln(os.Stderr, "dead=true push=delivered")
		}
		os.Exit(1)
	}
}

func argOr(args []string, i int, fallback string) string {
	if len(args) > i {
		return args[i]
	}
	return fallback
}

func usage() {
	fmt.Println(`asterisk-push-notify — acordador de softphone

  asterisk-push-notify <ramal> [contacts] [caller]        checa vivo; morto → push
  asterisk-push-notify --register <ramal> <token> [provider] [deviceId]
  asterisk-push-notify --list                              lista os tokens
  asterisk-push-notify --help

  <contacts> = saida de ${PJSIP_DIAL_CONTACTS(<ramal>)} (para o OPTIONS).

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

// sendPush envia o push (APNs/FCM) para os aparelhos registrados do ramal.
func sendPush(cfg Config, ramal, caller string) error {
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
	callID := randomHex(16)
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
