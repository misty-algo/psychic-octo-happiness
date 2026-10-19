// Package authhttp adapts authentication HTTP requests to the auth usecase.
package authhttp

import (
	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/auth/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// Handler adapts auth HTTP requests to the auth usecase.
type Handler struct {
	usecase       *usecase.Usecase
	secureCookies bool
}

// New creates an auth HTTP handler.
func New(uc *usecase.Usecase, secureCookies bool) *Handler {
	return &Handler{usecase: uc, secureCookies: secureCookies}
}

// Register mounts authentication routes on group.
func Register(group *echo.Group, handler *Handler) {
	group.POST("/auth/login", handler.Login)
	group.POST("/auth/logout", handler.Logout)
	group.POST("/auth/logout-others", handler.LogoutOthers)
	group.POST("/auth/password", handler.ChangePassword)
	group.POST("/auth/role", handler.SwitchRole)
	group.GET("/auth/me", handler.Me)
	group.PATCH("/auth/me", handler.UpdateProfile)
}

// Login handles administrator login.
func (h *Handler) Login(c *echo.Context) error {
	var req loginRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	output, err := h.usecase.Login(c.Request().Context(), usecase.LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		IP:        c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	})
	if err != nil {
		return err
	}
	if err := handlerSetLoginCookies(c, output, h.secureCookies); err != nil {
		return err
	}
	return httpresp.OK(c, output)
}

// Logout revokes the current login session on the server.
func (h *Handler) Logout(c *echo.Context) error {
	if err := h.usecase.Logout(c.Request().Context()); err != nil {
		return err
	}
	clearLoginCookies(c, h.secureCookies)
	return httpresp.OK(c, map[string]bool{"ok": true})
}

// LogoutOthers revokes the current administrator's other login sessions.
func (h *Handler) LogoutOthers(c *echo.Context) error {
	if err := h.usecase.LogoutOthers(c.Request().Context()); err != nil {
		return err
	}
	return httpresp.OK(c, map[string]bool{"ok": true})
}

// ChangePassword updates the current administrator password and revokes other
// login sessions for the same administrator.
func (h *Handler) ChangePassword(c *echo.Context) error {
	var req changePasswordRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.usecase.ChangePassword(c.Request().Context(), usecase.ChangePasswordInput{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); err != nil {
		return err
	}
	return httpresp.OK(c, map[string]bool{"ok": true})
}

// SwitchRole changes the active role for the current administrator.
func (h *Handler) SwitchRole(c *echo.Context) error {
	var req switchRoleRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	output, err := h.usecase.SwitchRole(c.Request().Context(), usecase.RoleSwitchInput{RoleID: req.RoleID})
	if err != nil {
		return err
	}
	return httpresp.OK(c, output)
}

// Me returns the authenticated administrator profile.
func (h *Handler) Me(c *echo.Context) error {
	user, err := h.usecase.CurrentUser(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, user)
}

// UpdateProfile updates the current administrator's profile fields.
func (h *Handler) UpdateProfile(c *echo.Context) error {
	var req updateProfileRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	user, err := h.usecase.UpdateProfile(c.Request().Context(), usecase.UpdateProfileInput{
		DisplayName: req.DisplayName,
		Email:       req.Email,
	})
	if err != nil {
		return err
	}
	return httpresp.OK(c, user)
}

func handlerSetLoginCookies(c *echo.Context, output usecase.LoginOutput, secure bool) error {
	if output.SessionToken == "" || output.SessionExpiresAt.IsZero() {
		return apperr.New(apperr.ErrInternalServer, "login session was not created")
	}
	SetLoginSessionCookie(c, output.SessionToken, output.SessionExpiresAt, secure)
	csrfToken, err := NewCSRFToken()
	if err != nil {
		return err
	}
	SetCSRFCookie(c, csrfToken, output.SessionExpiresAt, secure)
	return nil
}

func clearLoginCookies(c *echo.Context, secure bool) {
	ClearLoginSessionCookie(c, secure)
	ClearCSRFCookie(c, secure)
}
