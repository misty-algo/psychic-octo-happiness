package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/apitoken/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

const (
	rawSecretBytes = 32
	secretPrefix   = "ea_"
	displayChars   = 12
)

// ListTokens returns paginated API token metadata without secret hashes.
func (u *Usecase) ListTokens(ctx context.Context, input ListInput) (TokenListOutput, error) {
	if err := u.readyManagement(); err != nil {
		return TokenListOutput{}, err
	}
	filter, err := normalizeListInput(input)
	if err != nil {
		return TokenListOutput{}, err
	}
	if scopeErr := u.scopeListFilter(ctx, &filter); scopeErr != nil {
		return TokenListOutput{}, scopeErr
	}
	tokens, total, err := u.store.ListTokens(ctx, filter)
	if err != nil {
		return TokenListOutput{}, err
	}
	return TokenListOutput{
		Items:    mapTokens(tokens),
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

// CreateToken creates a token for the requested admin-role pair.
func (u *Usecase) CreateToken(ctx context.Context, input TokenInput) (CreatedToken, error) {
	if err := u.readyManagement(); err != nil {
		return CreatedToken{}, err
	}
	target, err := u.issueTarget(ctx, input)
	if err != nil {
		return CreatedToken{}, err
	}
	expiresAt, err := u.expiryFromInput(input)
	if err != nil {
		return CreatedToken{}, err
	}
	if expiryErr := u.validateExpiry(input.Active, expiresAt); expiryErr != nil {
		return CreatedToken{}, expiryErr
	}
	secret, prefix, hash, err := u.newSecretMaterial()
	if err != nil {
		return CreatedToken{}, err
	}
	token, err := domain.RestoreAPIToken(0, target.adminID, target.roleID, input.Name, input.Description, prefix, hash, input.Active, expiresAt, nil, time.Time{}, time.Time{})
	if err != nil {
		return CreatedToken{}, mapDomainError(err)
	}
	created, err := u.store.CreateToken(ctx, token)
	if err != nil {
		return CreatedToken{}, err
	}
	return CreatedToken{Token: fromToken(created), Secret: secret}, nil
}

// UpdateToken replaces mutable metadata while preserving the stored secret hash.
func (u *Usecase) UpdateToken(ctx context.Context, input UpdateTokenInput) (APIToken, error) {
	if err := u.readyManagement(); err != nil {
		return APIToken{}, err
	}
	existing, err := u.store.FindTokenByID(ctx, input.ID)
	if err != nil {
		return APIToken{}, err
	}
	if visibleErr := u.ensureTokenVisible(ctx, existing); visibleErr != nil {
		return APIToken{}, visibleErr
	}
	if expiryErr := u.validateExpiry(input.Active, input.ExpiresAt); expiryErr != nil {
		return APIToken{}, expiryErr
	}
	token, err := domain.RestoreAPIToken(
		existing.ID,
		existing.AdminID,
		existing.RoleID,
		input.Name,
		input.Description,
		existing.Prefix,
		existing.SecretHash,
		input.Active,
		input.ExpiresAt,
		existing.LastUsedAt,
		existing.CreatedAt,
		time.Time{},
	)
	if err != nil {
		return APIToken{}, mapDomainError(err)
	}
	updated, err := u.store.UpdateToken(ctx, token)
	if err != nil {
		return APIToken{}, err
	}
	return fromToken(updated), nil
}

// DeleteToken revokes an API token by id.
func (u *Usecase) DeleteToken(ctx context.Context, id int64) error {
	if err := u.readyManagement(); err != nil {
		return err
	}
	if id <= 0 {
		return apperr.NewBadRequest("invalid api token id")
	}
	existing, err := u.store.FindTokenByID(ctx, id)
	if err != nil {
		return err
	}
	if err := u.ensureTokenVisible(ctx, existing); err != nil {
		return err
	}
	return u.store.DeleteToken(ctx, id)
}

// Authenticate verifies a raw API token secret and returns its admin-role identity.
func (u *Usecase) Authenticate(ctx context.Context, secret string) (TokenIdentity, error) {
	if err := u.ready(); err != nil {
		return TokenIdentity{}, err
	}
	hash, err := domain.HashSecret(secret)
	if err != nil {
		return TokenIdentity{}, apperr.NewUnauthorized()
	}
	token, err := u.store.FindTokenByHash(ctx, hash)
	if err != nil {
		if isNotFound(err) {
			return TokenIdentity{}, apperr.NewUnauthorized()
		}
		return TokenIdentity{}, err
	}
	if !domain.HashMatches(token.SecretHash, hash) {
		return TokenIdentity{}, apperr.NewUnauthorized()
	}
	if !token.Active || tokenExpired(token, u.now()) {
		return TokenIdentity{}, apperr.NewUnauthorized()
	}
	if err := u.store.TouchLastUsed(ctx, token.ID, u.now()); err != nil {
		return TokenIdentity{}, err
	}
	return TokenIdentity{AdminID: token.AdminID, RoleID: token.RoleID}, nil
}

func (u *Usecase) ready() error {
	if u == nil || u.store == nil || u.now == nil || u.secretSource == nil {
		return apperr.New(apperr.ErrInternalServer, "api token dependencies are not configured")
	}
	return nil
}

func (u *Usecase) readyManagement() error {
	if err := u.ready(); err != nil {
		return err
	}
	if u.admins == nil || u.roles == nil {
		return apperr.New(apperr.ErrInternalServer, "api token management dependencies are not configured")
	}
	return nil
}

type issueTarget struct {
	adminID int64
	roleID  int64
}

func (u *Usecase) issueTarget(ctx context.Context, input TokenInput) (issueTarget, error) {
	currentAdminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return issueTarget{}, err
	}
	currentRoleID, err := requestctx.RequireRoleID(ctx)
	if err != nil {
		return issueTarget{}, err
	}
	target := issueTarget{adminID: coalesceID(input.AdminID, currentAdminID), roleID: coalesceID(input.RoleID, currentRoleID)}
	if target.adminID <= 0 || target.roleID <= 0 {
		return issueTarget{}, apperr.NewBadRequest("invalid api token identity")
	}
	view, err := u.roles.RoleView(ctx, currentRoleID)
	if err != nil {
		return issueTarget{}, err
	}
	if !view.IsSuper && (target.adminID != currentAdminID || target.roleID != currentRoleID) {
		return issueTarget{}, apperr.NewPermissionDenied("api_token", "issue")
	}
	admin, err := u.admins.AdminSnapshot(ctx, target.adminID)
	if err != nil {
		return issueTarget{}, err
	}
	if !admin.Active {
		return issueTarget{}, apperr.NewBadRequest("api token admin is disabled")
	}
	if !containsID(admin.RoleIDs, target.roleID) {
		return issueTarget{}, apperr.NewBadRequest("api token admin does not have the role")
	}
	return target, nil
}

func (u *Usecase) scopeListFilter(ctx context.Context, filter *ListFilter) error {
	currentAdminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	currentRoleID, err := requestctx.RequireRoleID(ctx)
	if err != nil {
		return err
	}
	view, err := u.roles.RoleView(ctx, currentRoleID)
	if err != nil {
		return err
	}
	if !view.IsSuper {
		filter.AdminID = currentAdminID
		return nil
	}
	if filter.AdminID < 0 {
		return apperr.NewBadRequest("invalid api token admin id")
	}
	return nil
}

func (u *Usecase) ensureTokenVisible(ctx context.Context, token domain.APIToken) error {
	currentAdminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	currentRoleID, err := requestctx.RequireRoleID(ctx)
	if err != nil {
		return err
	}
	view, err := u.roles.RoleView(ctx, currentRoleID)
	if err != nil {
		return err
	}
	if view.IsSuper || token.AdminID == currentAdminID {
		return nil
	}
	return apperr.NewPermissionDenied("api_token", strconv.FormatInt(token.ID, 10))
}

func (u *Usecase) expiryFromInput(input TokenInput) (*time.Time, error) {
	if input.Days > 0 {
		if input.Days > maxTokenDays {
			return nil, apperr.NewBadRequest("api token days must be between 1 and 365")
		}
		expiresAt := u.now().Add(time.Duration(input.Days) * 24 * time.Hour)
		return &expiresAt, nil
	}
	if input.Active && input.ExpiresAt == nil {
		return nil, apperr.NewBadRequest("api token days must be between 1 and 365")
	}
	return input.ExpiresAt, nil
}

func (u *Usecase) validateExpiry(active bool, expiresAt *time.Time) error {
	if active && expiresAt != nil && !expiresAt.After(u.now()) {
		return apperr.NewBadRequest("api token expires_at must be in the future")
	}
	return nil
}

func coalesceID(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func (u *Usecase) newSecretMaterial() (string, string, string, error) {
	secret, err := u.secretSource()
	if err != nil {
		return "", "", "", fmt.Errorf("generate api token secret: %w", err)
	}
	hash, err := domain.HashSecret(secret)
	if err != nil {
		return "", "", "", mapDomainError(err)
	}
	prefix := secret
	if len(prefix) > displayChars {
		prefix = prefix[:displayChars]
	}
	return secret, prefix, hash, nil
}

func generateSecret() (string, error) {
	raw := make([]byte, rawSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read secure random bytes: %w", err)
	}
	return secretPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func containsID(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func normalizeListInput(input ListInput) (ListFilter, error) {
	window, err := pagination.Normalize(input.Page, input.PageSize, pagination.DefaultOptions)
	if err != nil {
		return ListFilter{}, apperr.NewBadRequest("invalid pagination")
	}
	return ListFilter{Window: window, AdminID: input.AdminID, Active: input.Active}, nil
}

func tokenExpired(token domain.APIToken, now time.Time) bool {
	return token.ExpiresAt != nil && !now.Before(*token.ExpiresAt)
}

func mapTokens(tokens []domain.APIToken) []APIToken {
	out := make([]APIToken, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, fromToken(token))
	}
	return out
}

func isNotFound(err error) bool {
	def, ok := apperr.ParseRegistered(err)
	return ok && def.Kind == apperr.KindNotFound
}

func fromToken(token domain.APIToken) APIToken {
	return APIToken{
		ID:          token.ID,
		AdminID:     token.AdminID,
		RoleID:      token.RoleID,
		Name:        token.Name,
		Description: token.Description,
		Prefix:      token.Prefix,
		Active:      token.Active,
		ExpiresAt:   copyTimePtr(token.ExpiresAt),
		LastUsedAt:  copyTimePtr(token.LastUsedAt),
		CreatedAt:   token.CreatedAt,
		UpdatedAt:   token.UpdatedAt,
	}
}

func copyTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func mapDomainError(err error) error {
	for _, entry := range domainErrorMessages {
		if errors.Is(err, entry.err) {
			return apperr.NewBadRequest(entry.message)
		}
	}
	return err
}

var domainErrorMessages = []struct {
	err     error
	message string
}{
	{domain.ErrInvalidTokenID, "invalid api token id"},
	{domain.ErrInvalidTokenName, "invalid api token name"},
	{domain.ErrInvalidTokenDesc, "invalid api token description"},
	{domain.ErrInvalidTokenPrefix, "invalid api token prefix"},
	{domain.ErrInvalidTokenHash, "invalid api token hash"},
	{domain.ErrInvalidTokenSecret, "invalid api token secret"},
	{domain.ErrInvalidAdminID, "invalid api token admin id"},
	{domain.ErrInvalidRoleID, "invalid api token role id"},
}
