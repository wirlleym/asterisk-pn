package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

const optionsTimeout = 2 * time.Second

// checkAlive envia um SIP OPTIONS para cada contato e verifica se o app responde.
// Retorna true se algum contato respondeu (app vivo); false se nenhum (app morto).
func checkAlive(contacts string) bool {
	contacts = strings.TrimSpace(contacts)
	if contacts == "" {
		return false
	}
	for _, c := range strings.Split(contacts, "&") {
		if optionsPing(strings.TrimSpace(c)) {
			return true
		}
	}
	return false
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// optionsPing manda um OPTIONS para um contato "sip:user@host:port" e espera
// uma resposta. Qualquer resposta = app vivo.
func optionsPing(contact string) bool {
	host, port, user := parseContact(contact)
	if host == "" || port == "" {
		return false
	}
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("udp", addr, 1500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(optionsTimeout))

	if _, err := conn.Write([]byte(buildOptions(host, port, user))); err != nil {
		return false
	}
	buf := make([]byte, 2048)
	_, err = conn.Read(buf)
	return err == nil
}

// parseContact extrai host, porta e usuario de um contato SIP.
func parseContact(contact string) (host, port, user string) {
	s := contact
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, ";?"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		user = s[:i]
		s = s[i+1:]
	}
	host = s
	port = "5060"
	if i := strings.LastIndex(s, ":"); i >= 0 {
		p := s[i+1:]
		if allDigits(p) {
			host = s[:i]
			port = p
		}
	}
	return host, port, user
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// buildOptions monta um SIP OPTIONS simples.
func buildOptions(host, port, user string) string {
	branch := "z9hG4bK" + randomHex(8)
	tag := randomHex(8)
	callID := randomHex(16)
	via := net.JoinHostPort(host, port)
	uri := fmt.Sprintf("sip:%s@%s", user, via)
	return fmt.Sprintf(
		"OPTIONS %s SIP/2.0\r\n"+
			"Via: SIP/2.0/UDP %s;branch=%s;rport\r\n"+
			"From: <sip:asterisk@%s>;tag=%s\r\n"+
			"To: <%s>\r\n"+
			"Call-ID: %s@%s\r\n"+
			"CSeq: 1 OPTIONS\r\n"+
			"Max-Forwards: 70\r\n"+
			"Content-Length: 0\r\n\r\n",
		uri, via, branch, host, tag, uri, callID, host,
	)
}
