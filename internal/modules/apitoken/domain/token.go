// Package domain contains API token validation and secret handling rules.
package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

const (
	tokenHashLength = sha256.Size * 2
	maxSecretLength = 256
)

// API token validation errors.
var (
	ErrInvalidTokenID     = errors.New("invalid api token id")
	ErrInvalidTokenName   = errors.New("invalid api token name")
	ErrInvalidTokenDesc   = errors.New("invalid api token description")
	ErrInvalidTokenPrefix = errors.New("invalid api token prefix")
	ErrInvalidTokenHash   = errors.New("invalid api token hash")
	ErrInvalidTokenSecret = errors.New("invalid api token secret")
	ErrInvalidAdminID     = errors.New("invalid api token admin id")
	ErrInvalidRoleID      = errors.New("invalid api token role id")
)

// APIToken is a validated API credential record. It never carries the raw
// secret because the plain token is only safe to return once at creation time.
type APIToken struct {
	ID          int64
	AdminID     int64
	RoleID      int64
	Name        string
	Description string
	Prefix      string
	SecretHash  string
	Active      bool
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RestoreAPIToken rebuilds a token from a trusted store representation.
func RestoreAPIToken(id, adminID, roleID int64, name, description, prefix, secretHash string, active bool, expiresAt, lastUsedAt *time.Time, createdAt, updatedAt time.Time) (APIToken, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	prefix = strings.TrimSpace(prefix)
	secretHash = strings.ToLower(strings.TrimSpace(secretHash))

	if id < 0 {
		return APIToken{}, ErrInvalidTokenID
	}
	if adminID <= 0 {
		return APIToken{}, ErrInvalidAdminID
	}
	if roleID <= 0 {
		return APIToken{}, ErrInvalidRoleID
	}
	if name == "" || len(name) > 80 {
		return APIToken{}, ErrInvalidTokenName
	}
	if len(description) > 240 {
		return APIToken{}, ErrInvalidTokenDesc
	}
	if !isDisplayPrefix(prefix) {
		return APIToken{}, ErrInvalidTokenPrefix
	}
	if !isSHA256Hex(secretHash) {
		return APIToken{}, ErrInvalidTokenHash
	}
	return APIToken{
		ID:          id,
		AdminID:     adminID,
		RoleID:      roleID,
		Name:        name,
		Description: description,
		Prefix:      prefix,
		SecretHash:  secretHash,
		Active:      active,
		ExpiresAt:   copyTimePtr(expiresAt),
		LastUsedAt:  copyTimePtr(lastUsedAt),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// HashSecret returns the stable lookup hash for a raw API token secret.
func HashSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" || len(secret) > maxSecretLength {
		return "", ErrInvalidTokenSecret
	}
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:]), nil
}

// HashMatches compares two token hashes without leaking equality through timing.
func HashMatches(storedHash, candidateHash string) bool {
	storedHash = strings.ToLower(strings.TrimSpace(storedHash))
	candidateHash = strings.ToLower(strings.TrimSpace(candidateHash))
	if !isSHA256Hex(storedHash) || !isSHA256Hex(candidateHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(candidateHash)) == 1
}

func isDisplayPrefix(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

func isSHA256Hex(value string) bool {
	if len(value) != tokenHashLength {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func copyTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
