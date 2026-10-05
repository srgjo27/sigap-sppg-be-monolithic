package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
)

// Handler decodes HTTP, calls application service, maps results.
type Handler struct {
	svc *application.Service
}

// NewHandler builds a handler around the application service.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func clientIP(c *gin.Context) *string {
	ip := c.ClientIP()
	if ip == "" {
		return nil
	}
	return &ip
}

func userAgent(c *gin.Context) *string {
	ua := c.Request.UserAgent()
	if ua == "" {
		return nil
	}
	return &ua
}

// CreateUser handles POST /users (MVP-001.1).
func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	claims, _ := CurrentUser(c)
	res, err := h.svc.CreateUser(c.Request.Context(), claims, domain.CreateUserInput{
		Nama:         req.Nama,
		Email:        req.Email,
		NoHP:         req.NoHP,
		Peran:        domain.Role(req.Peran),
		SPPGID:       req.SPPGID,
		SekolahID:    req.SekolahID,
		PasswordAwal: req.PasswordAwal,
	}, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CreateUserResponse{User: toUserResponse(res.User), GeneratedPassword: res.GeneratedPassword})
}

// Login handles POST /auth/login (MVP-001.2).
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, clientIP(c), userAgent(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, LoginResponse{
		AccessToken:        res.AccessToken,
		RefreshToken:       res.RefreshToken,
		ExpiresIn:          res.ExpiresIn,
		WajibGantiPassword: res.WajibGantiPassword,
		User:               toLoginBrief(res.User),
	})
}

// Refresh handles POST /auth/refresh (MVP-001.3).
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"refresh_token": "required"}})
		return
	}
	res, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken, clientIP(c), userAgent(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, RefreshResponse{AccessToken: res.AccessToken, RefreshToken: res.RefreshToken, ExpiresIn: res.ExpiresIn})
}

// Logout handles POST /auth/logout (MVP-001.3).
func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"refresh_token": "required"}})
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken, clientIP(c)); err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Me handles GET /auth/me (MVP-001.4).
func (h *Handler) Me(c *gin.Context) {
	claims, ok := CurrentUser(c)
	if !ok {
		MapError(c, domain.ErrUnauthorized)
		return
	}
	u, perms, err := h.svc.Me(c.Request.Context(), claims.UserID)
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, MeResponse{User: toUserResponse(u), Permissions: perms})
}

// UpdateUser handles PATCH /users/:id (MVP-001.5).
func (h *Handler) UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrNotFound)
		return
	}
	claims, _ := CurrentUser(c)
	var peran *domain.Role
	if req.Peran != nil {
		r := domain.Role(*req.Peran)
		peran = &r
	}
	updated, err := h.svc.UpdateUser(c.Request.Context(), claims, id, domain.UpdateUserInput{
		Nama:      req.Nama,
		NoHP:      req.NoHP,
		Peran:     peran,
		SPPGID:    req.SPPGID,
		SekolahID: req.SekolahID,
		Aktif:     req.Aktif,
	}, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(updated))
}

// ChangePassword handles PUT /auth/password (MVP-001.6).
func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	claims, ok := CurrentUser(c)
	if !ok {
		MapError(c, domain.ErrUnauthorized)
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), claims.UserID, req.PasswordLama, req.PasswordBaru, clientIP(c)); err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

// ListAuditLogs handles GET /audit-logs (MVP-001.7).
func (h *Handler) ListAuditLogs(c *gin.Context) {
	claims, ok := CurrentUser(c)
	if !ok {
		MapError(c, domain.ErrUnauthorized)
		return
	}
	filter := domain.AuditFilter{}
	if v := c.Query("aksi"); v != "" {
		filter.Aksi = &v
	}
	if v := c.Query("tabel"); v != "" {
		filter.Tabel = &v
	}
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.UserID = &id
		}
	}
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Page = n
		}
	}
	if v := c.Query("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	entries, total, err := h.svc.ListAuditLogs(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}
	filter.Normalize()
	items := make([]AuditLogItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, toAuditItem(e))
	}
	c.JSON(http.StatusOK, AuditLogListResponse{
		Data: items,
		Meta: PageMeta{Page: filter.Page, PerPage: filter.Limit, Total: total},
	})
}
