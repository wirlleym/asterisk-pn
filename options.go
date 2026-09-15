package main

import (
	"crypto/rand"
	"crypto/tls"
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

// optionsPing manda um OPTIONS para um contato e espera resposta (qualquer
// resposta = app vivo). Usa o transporte do contato (udp/tcp/tls).
func optionsPing(contact string) bool {
	host, port, user, transport := parseContact(contact)
	if host == "" || port == "" {
		return false
	}
	msg := buildOptions(transport, host, port, user)
	return pingTransport(transport, host, port, msg)
}

// pingTransport envia a mensagem SIP pelo transporte certo.
func pingTransport(transport, host, port, msg string) bool {
	switch transport {
	case "tcp", "tls":
		return pingStream(transport, host, port, msg)
	default: // udp
		return pingUDP(host, port, msg)
	}
}

func pingUDP(host, port, msg string) bool {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("udp", addr, 1500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(optionsTimeout))

	if _, err := conn.Write([]byte(msg)); err != nil {
		return false
	}
	buf := make([]byte, 2048)
	_, err = conn.Read(buf)
	return err == nil
}

func pingStream(transport, host, port, msg string) bool {
	addr := net.JoinHostPort(host, port)
	dialer := &net.Dialer{Timeout: 1500 * time.Millisecond}
	var conn net.Conn
	var err error
	if transport == "tls" {
		// Checagem de aliveness: o app pode ter certificado proprio; nao
		// validamos a cadeia, so queremos saber se responde.
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(optionsTimeout))

	if _, err := conn.Write([]byte(msg)); err != nil {
		return false
	}
	buf := make([]byte, 2048)
	_, err = conn.Read(buf)
	return err == nil
}

// parseContact extrai host, porta, usuario e transporte de um contato SIP.
func parseContact(contact string) (host, port, user, transport string) {
	s := contact
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// parametros (;transport=...)
	if i := strings.Index(s, ";"); i >= 0 {
		params := s[i+1:]
		s = s[:i]
		for _, p := range strings.Split(params, ";") {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "transport=") {
				transport = strings.ToLower(strings.TrimPrefix(p, "transport="))
			}
		}
	}
	if i := strings.Index(s, "?"); i >= 0 {
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
	if transport == "" {
		transport = "udp"
	}
	return host, port, user, transport
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
func buildOptions(transport, host, port, user string) string {
	branch := "z9hG4bK" + randomHex(8)
	tag := randomHex(8)
	callID := randomHex(16)
	via := net.JoinHostPort(host, port)
	uri := fmt.Sprintf("sip:%s@%s", user, via)
	viaProto := strings.ToUpper(transport)
	return fmt.Sprintf(
		"OPTIONS %s SIP/2.0\r\n"+
			"Via: SIP/2.0/%s %s;branch=%s;rport\r\n"+
			"From: <sip:asterisk@%s>;tag=%s\r\n"+
			"To: <%s>\r\n"+
			"Call-ID: %s@%s\r\n"+
			"CSeq: 1 OPTIONS\r\n"+
			"Max-Forwards: 70\r\n"+
			"Content-Length: 0\r\n\r\n",
		uri, viaProto, via, branch, host, tag, uri, callID, host,
	)
}
