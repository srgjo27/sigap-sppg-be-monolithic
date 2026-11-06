package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// CreateUser handles POST /users.
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

// Login handles POST /auth/login.
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

// Refresh handles POST /auth/refresh.
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

// Logout handles POST /auth/logout.
// Without query params it revokes the refresh token in the body.
// With ?all=true it revokes every session of the caller.
// Success is HTTP 204 with no body.
func (h *Handler) Logout(c *gin.Context) {
	claims, ok := CurrentUser(c)
	if !ok {
		MapError(c, domain.ErrUnauthorized)
		return
	}
	if c.Query("all") == "true" {
		if err := h.svc.LogoutAll(c.Request.Context(), claims.UserID, clientIP(c)); err != nil {
			MapError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
		return
	}
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"refresh_token": "required"}})
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken, clientIP(c)); err != nil {
		MapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Me handles GET /auth/me.
func (h *Handler) Me(c *gin.Context) {
	claims, ok := CurrentUser(c)
	if !ok {
		MapError(c, domain.ErrUnauthorized)
		return
	}
	profile, err := h.svc.Me(c.Request.Context(), claims.UserID)
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMeResponse(profile))
}

// UpdateUser handles PATCH /users/:id.
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
	res, err := h.svc.UpdateUser(c.Request.Context(), claims, id, domain.UpdateUserInput{
		Nama:          req.Nama,
		NoHP:          req.NoHP,
		Peran:         peran,
		SPPGID:        req.SPPGID,
		SekolahID:     req.SekolahID,
		Aktif:         req.Aktif,
		ResetPassword: req.ResetPassword,
	}, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, UpdateUserResponse{User: toUserResponse(res.User), PasswordSementara: res.TempPassword})
}

// ListUsers handles GET /users.
func (h *Handler) ListUsers(c *gin.Context) {
	claims, ok := CurrentUser(c)
	if !ok {
		MapError(c, domain.ErrUnauthorized)
		return
	}
	filter := domain.UserFilter{}
	if v := c.Query("q"); v != "" {
		filter.Q = &v
	}
	if v := c.Query("peran"); v != "" {
		for _, part := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				filter.Peran = append(filter.Peran, domain.Role(trimmed))
			}
		}
	}
	if v := c.Query("aktif"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"aktif": "must be a boolean"}})
			return
		}
		filter.Aktif = &parsed
	}
	if v := c.Query("sppg_id"); v != "" {
		if strings.EqualFold(v, "null") {
			filter.NullSPPG = true
		} else {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"sppg_id": "must be an integer or null"}})
				return
			}
			filter.SPPGID = &id
		}
	}
	if v := c.Query("sekolah_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"sekolah_id": "must be an integer"}})
			return
		}
		filter.SekolahID = &id
	}
	if v := c.Query("sort"); v != "" {
		filter.Sort = v
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
	views, total, err := h.svc.ListUsers(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}
	filter.Normalize()
	items := make([]UserListItem, 0, len(views))
	for _, vw := range views {
		items = append(items, toUserListItem(vw))
	}
	c.JSON(http.StatusOK, UserListResponse{
		Data: items,
		Meta: PageMeta{Page: filter.Page, PerPage: filter.Limit, Total: total},
	})
}

// GetUser handles GET /users/:id.
func (h *Handler) GetUser(c *gin.Context) {
	claims, ok := CurrentUser(c)
	if !ok {
		MapError(c, domain.ErrUnauthorized)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrNotFound)
		return
	}
	view, err := h.svc.GetUser(c.Request.Context(), claims, id)
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserDetailResponse(view))
}

// ChangePassword handles PUT /auth/password.
// Success is HTTP 204 with no body.
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
	c.Status(http.StatusNoContent)
}

// ListAuditLogs handles GET /audit-logs.
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
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"user_id": "must be an integer"}})
			return
		}
		filter.UserID = &id
	}
	if v := c.Query("record_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"record_id": "must be an integer"}})
			return
		}
		filter.RecordID = &id
	}
	if v := c.Query("dari"); v != "" {
		ts, err := parseAuditTime(v, true)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"dari": "use RFC3339 or YYYY-MM-DD"}})
			return
		}
		filter.Dari = &ts
	}
	if v := c.Query("sampai"); v != "" {
		ts, err := parseAuditTime(v, false)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"sampai": "use RFC3339 or YYYY-MM-DD"}})
			return
		}
		filter.Sampai = &ts
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
	views, total, err := h.svc.ListAuditLogs(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}
	filter.Normalize()
	items := make([]AuditLogItem, 0, len(views))
	for _, vw := range views {
		items = append(items, toAuditItem(vw))
	}
	c.JSON(http.StatusOK, AuditLogListResponse{
		Data: items,
		Meta: PageMeta{Page: filter.Page, PerPage: filter.Limit, Total: total},
	})
}

// parseAuditTime accepts RFC3339 or a YYYY-MM-DD date. Date-only dari is the
// start of day; date-only sampai is the end of day.
func parseAuditTime(raw string, isDari bool) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	if d, err := time.Parse("2006-01-02", raw); err == nil {
		if isDari {
			return d, nil
		}
		return d.Add(24*time.Hour - time.Nanosecond), nil
	}
	return time.Time{}, errors.New("invalid audit time")
}
