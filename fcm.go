package main

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
	ProjectID   string `json:"project_id"`
}

// fcmSender envia mensagens data-only via FCM HTTP v1 (OAuth2 com a service account).
type fcmSender struct {
	account   *serviceAccount
	cachedTok string
	cachedAt  time.Time
}

func newFCMSender(cfg Config) *fcmSender {
	s := &fcmSender{}
	if cfg.FCMServiceAccount != "" {
		if data, err := os.ReadFile(cfg.FCMServiceAccount); err == nil {
			var acc serviceAccount
			if json.Unmarshal(data, &acc) == nil {
				s.account = &acc
			}
		}
	}
	return s
}

func (f *fcmSender) accessToken() (string, error) {
	if f.account == nil {
		return "", fmt.Errorf("no service account")
	}
	if f.cachedTok != "" && time.Since(f.cachedAt) < 50*time.Minute {
		return f.cachedTok, nil
	}
	now := time.Now().Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(
		`{"iss":%q,"scope":"https://www.googleapis.com/auth/firebase.messaging","aud":%q,"iat":%d,"exp":%d}`,
		f.account.ClientEmail, f.account.TokenURI, now, now+3600)))
	signingInput := header + "." + claims
	block, _ := pem.Decode([]byte(f.account.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("invalid service account key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("service account key is not RSA")
	}
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(nil, rsaKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	assertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)
	resp, err := http.PostForm(f.account.TokenURI, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var parsed2 struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &parsed2); err != nil || parsed2.AccessToken == "" {
		return "", fmt.Errorf("token exchange failed")
	}
	f.cachedTok = parsed2.AccessToken
	f.cachedAt = time.Now()
	return f.cachedTok, nil
}

func (f *fcmSender) Send(device DeviceRecord, callID, handle string, ttlSeconds int) (bool, string) {
	if f.account == nil {
		return false, "fcm_credentials_missing"
	}
	token, err := f.accessToken()
	if err != nil {
		return false, "fcm_auth_error"
	}
	// Somente dados (data-only), prioridade HIGH, TTL curto, SEM "notification".
	payload := map[string]any{
		"message": map[string]any{
			"token": device.Token,
			"android": map[string]any{
				"priority": "HIGH",
				"ttl":      fmt.Sprintf("%ds", ttlSeconds),
			},
			"data": map[string]string{
				"type":    "incoming_call",
				"call_id": callID,
				"caller":  handle,
			},
		},
	}
	body, _ := json.Marshal(payload)
	endpoint := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", f.account.ProjectID)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, "fcm_request_error"
	}
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "fcm_request_error"
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		return true, ""
	}
	reason := strings.TrimSpace(string(respBody))
	if reason == "" {
		reason = fmt.Sprintf("fcm_http_%d", resp.StatusCode)
	}
	return false, reason
}
