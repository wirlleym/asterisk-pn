package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// apnsSender envia VoIP push para a Apple (HTTP/2 + JWT ES256 assinado com o .p8).
type apnsSender struct {
	cfg        Config
	cachedAuth string
	cachedAt   time.Time
}

func newAPNSSender(cfg Config) *apnsSender { return &apnsSender{cfg: cfg} }

func (a *apnsSender) authorization() (string, error) {
	if a.cachedAuth != "" && time.Since(a.cachedAt) < 50*time.Minute {
		return a.cachedAuth, nil
	}
	keyData, err := os.ReadFile(a.cfg.APNSKeyPath)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(keyData)
	if block == nil {
		return "", fmt.Errorf("invalid p8")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	ecKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("p8 is not an EC key")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","kid":"` + a.cfg.APNSKeyID + `"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(
		`{"iss":"` + a.cfg.APNSTeamID + `","iat":` + strconv.FormatInt(time.Now().Unix(), 10) + `}`))
	signingInput := header + "." + claims
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, ecKey, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64) // JWT ES256 = R||S, 32 bytes cada
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
	a.cachedAuth = token
	a.cachedAt = time.Now()
	return token, nil
}

func (a *apnsSender) Send(device DeviceRecord, callID, handle string, ttlSeconds int) (bool, string) {
	if !strings.HasSuffix(a.cfg.APNSTopic, ".voip") {
		return false, "apns_topic_must_end_with_voip"
	}
	if a.cfg.APNSKeyPath == "" || a.cfg.APNSKeyID == "" || a.cfg.APNSTeamID == "" {
		return false, "apns_credentials_missing"
	}
	auth, err := a.authorization()
	if err != nil {
		return false, "apns_auth_error"
	}
	host := "https://api.push.apple.com"
	if a.cfg.APNSEnvironment == "sandbox" {
		host = "https://api.sandbox.push.apple.com"
	}
	body := fmt.Sprintf(`{"callId":%q,"handle":%q,"type":"incoming_call"}`, callID, handle)
	req, err := http.NewRequest(http.MethodPost, host+"/3/device/"+device.Token, strings.NewReader(body))
	if err != nil {
		return false, "apns_request_error"
	}
	req.Header.Set("authorization", "bearer "+auth)
	req.Header.Set("apns-topic", a.cfg.APNSTopic)
	req.Header.Set("apns-push-type", "voip")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-expiration", strconv.FormatInt(time.Now().Unix()+int64(ttlSeconds), 10))
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "apns_request_error"
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		return true, ""
	}
	reason := strings.TrimSpace(string(respBody))
	if reason == "" {
		reason = fmt.Sprintf("apns_http_%d", resp.StatusCode)
	}
	return false, reason
}
