package api

import (
	"strings"
	"testing"
	"time"
)

func TestDelegationBindsActorRequestAudienceAndLifetime(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	now := time.Unix(1000, 0)
	body := []byte(`{"world":"one"}`)
	token, err := SignDelegation(key, "User@Example.test", "canon-tools", "POST", "/tools", body, now)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := VerifyDelegation(key, token, "canon-tools", "POST", "/tools", body, now)
	if err != nil || actor != "user@example.test" {
		t.Fatalf("valid: %s %v", actor, err)
	}
	for _, c := range []struct {
		audience, method, path string
		body                   []byte
		at                     time.Time
	}{
		{"publish", "POST", "/tools", body, now}, {"canon-tools", "GET", "/tools", body, now}, {"canon-tools", "POST", "/publish", body, now},
		{"canon-tools", "POST", "/tools", []byte(`{"world":"two"}`), now}, {"canon-tools", "POST", "/tools", body, now.Add(time.Minute)},
		{"canon-tools", "POST", "/tools", body, now.Add(-6 * time.Second)},
	} {
		if _, err := VerifyDelegation(key, token, c.audience, c.method, c.path, c.body, c.at); err == nil {
			t.Fatal("accepted altered/expired request")
		}
	}
	if _, err := VerifyDelegation([]byte(strings.Repeat("x", 32)), token, "canon-tools", "POST", "/tools", body, now); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := SignDelegation([]byte("short"), actor, "canon-tools", "POST", "/tools", body, now); err == nil {
		t.Fatal("weak configuration accepted")
	}
}
