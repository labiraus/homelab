package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Delegations bind an authenticated actor to an exact request and audience.
// They convey no role; recipients must authorise each operation independently.
type delegation struct {
	Actor    string `json:"actor"`
	Audience string `json:"audience"`
	Request  string `json:"request"`
	Issued   int64  `json:"issued"`
	Expires  int64  `json:"expires"`
}

func SignDelegation(key []byte, actor, audience, method, path string, body []byte, now time.Time) (string, error) {
	actor = strings.ToLower(strings.TrimSpace(actor))
	if len(key) < 32 || actor == "" || len(actor) > 320 || audience == "" {
		return "", errors.New("invalid delegation configuration")
	}
	d := delegation{actor, audience, requestDigest(method, path, body), now.Unix(), now.Add(time.Minute).Unix()}
	raw, _ := json.Marshal(d)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyDelegation(key []byte, token, audience, method, path string, body []byte, now time.Time) (string, error) {
	invalid := errors.New("invalid delegated identity")
	if len(key) < 32 || len(token) > 2048 {
		return "", invalid
	}
	payload, signature, ok := strings.Cut(token, ".")
	if !ok {
		return "", invalid
	}
	signatureBytes, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", invalid
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	if !hmac.Equal(signatureBytes, mac.Sum(nil)) {
		return "", invalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", invalid
	}
	var d delegation
	if json.Unmarshal(raw, &d) != nil || d.Actor == "" || len(d.Actor) > 320 || d.Actor != strings.ToLower(strings.TrimSpace(d.Actor)) || d.Audience != audience || d.Request != requestDigest(method, path, body) || d.Expires <= now.Unix() || d.Issued > now.Add(5*time.Second).Unix() || d.Expires <= d.Issued || d.Expires-d.Issued > 60 {
		return "", invalid
	}
	return d.Actor, nil
}

func requestDigest(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method + "\n" + path + "\n"))
	h.Write(body)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
