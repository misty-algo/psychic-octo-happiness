package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRestoreAPITokenValidatesHashAndTrimsFields(t *testing.T) {
	hash, err := HashSecret("ea_test_secret")
	if err != nil {
		t.Fatalf("HashSecret() error = %v", err)
	}
	token, err := RestoreAPIToken(1, 2, 3, "  Deploy Bot  ", "  CI token  ", "ea_test_sec", hash, true, nil, nil, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreAPIToken() error = %v", err)
	}
	if token.Name != "Deploy Bot" {
		t.Fatalf("Name = %q, want Deploy Bot", token.Name)
	}
	if token.Description != "CI token" {
		t.Fatalf("Description = %q, want CI token", token.Description)
	}
	if token.SecretHash != hash {
		t.Fatalf("SecretHash = %q, want %q", token.SecretHash, hash)
	}
}

func TestRestoreAPITokenRejectsInvalidHash(t *testing.T) {
	_, err := RestoreAPIToken(1, 2, 3, "Deploy Bot", "", "ea_test_sec", "not-a-hash", true, nil, nil, fixedTime(), fixedTime())
	if !errors.Is(err, ErrInvalidTokenHash) {
		t.Fatalf("RestoreAPIToken() error = %v, want %v", err, ErrInvalidTokenHash)
	}
}

func TestHashMatchesUsesNormalizedHashes(t *testing.T) {
	hash, err := HashSecret("ea_test_secret")
	if err != nil {
		t.Fatalf("HashSecret() error = %v", err)
	}
	if !HashMatches(hash, hash) {
		t.Fatal("HashMatches() = false, want true")
	}
	if HashMatches(hash, "bad") {
		t.Fatal("HashMatches() = true for invalid candidate, want false")
	}
}

func fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
}
