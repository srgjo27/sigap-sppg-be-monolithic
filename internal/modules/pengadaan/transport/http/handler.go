package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

type Handler struct {
	svc *application.Service
}

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

// CreatePemasok handles POST /api/v1/pemasok
func (h *Handler) CreatePemasok(c *gin.Context) {
	var req CreatePemasokRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	in := domain.CreatePemasokInput{
		Nama:       req.Nama,
		JenisUsaha: req.JenisUsaha,
		Kontak:     req.Kontak,
		NoHP:       req.NoHP,
		Alamat:     req.Alamat,
		SPPGID:     req.SPPGID,
	}
	res, err := h.svc.CreatePemasok(c.Request.Context(), claims, in, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPemasokResponse(res.Pemasok))
}

// GetPemasok handles GET /api/v1/pemasok/{id}
func (h *Handler) GetPemasok(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPemasokNotFound)
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.GetPemasok(c.Request.Context(), claims, id)
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPemasokDetailResponse(res.Detail))
}

// UpdatePemasok handles PATCH /api/v1/pemasok/{id}
func (h *Handler) UpdatePemasok(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPemasokNotFound)
		return
	}
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}

	var req UpdatePemasokRequest
	if v, ok := raw["nama"]; ok {
		if err := unmarshalRawMessage(v, &req.Nama); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"nama": "must be a string"}})
			return
		}
	}
	if v, ok := raw["jenis_usaha"]; ok {
		if err := unmarshalRawMessage(v, &req.JenisUsaha); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"jenis_usaha": "must be a string"}})
			return
		}
	}
	if v, ok := raw["kontak"]; ok {
		if string(v) == "null" {
			s := ""
			req.Kontak = &s
		} else {
			if err := unmarshalRawMessage(v, &req.Kontak); err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"kontak": "must be a string"}})
				return
			}
		}
	}
	if v, ok := raw["no_hp"]; ok {
		if err := unmarshalRawMessage(v, &req.NoHP); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"no_hp": "must be a string"}})
			return
		}
	}
	if v, ok := raw["alamat"]; ok {
		if string(v) == "null" {
			s := ""
			req.Alamat = &s
		} else {
			if err := unmarshalRawMessage(v, &req.Alamat); err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"alamat": "must be a string"}})
				return
			}
		}
	}
	if v, ok := raw["aktif"]; ok {
		if err := unmarshalRawMessage(v, &req.Aktif); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"aktif": "must be a boolean"}})
			return
		}
	}
	if v, ok := raw["alasan"]; ok {
		if string(v) == "null" {
			s := ""
			req.Alasan = &s
		} else {
			if err := unmarshalRawMessage(v, &req.Alasan); err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"alasan": "must be a string"}})
				return
			}
		}
	}
	if v, ok := raw["sppg_id"]; ok {
		if err := unmarshalRawMessage(v, &req.SPPGID); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"sppg_id": "must be an integer"}})
			return
		}
	}

	claims, _ := authhttp.CurrentUser(c)
	in := domain.UpdatePemasokInput{
		Nama:       req.Nama,
		JenisUsaha: req.JenisUsaha,
		Kontak:     req.Kontak,
		NoHP:       req.NoHP,
		Alamat:     req.Alamat,
		Aktif:      req.Aktif,
		Alasan:     req.Alasan,
		SPPGID:     req.SPPGID,
	}
	res, err := h.svc.UpdatePemasok(c.Request.Context(), claims, id, in, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPemasokResponse(res.Pemasok))
}

// ListPemasok handles GET /api/v1/pemasok
func (h *Handler) ListPemasok(c *gin.Context) {
	claims, _ := authhttp.CurrentUser(c)
	filter := domain.PemasokFilter{}
	if v := c.Query("q"); v != "" {
		filter.Q = &v
	}
	if v := c.Query("jenis_usaha"); v != "" {
		filter.JenisUsaha = &v
	}
	if v := c.Query("aktif"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"aktif": "must be a boolean"}})
			return
		}
		filter.Aktif = &parsed
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
	res, err := h.svc.ListPemasok(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}
	data := make([]PemasokResponse, 0, len(res.Data))
	for i := range res.Data {
		data = append(data, toPemasokResponse(&res.Data[i]))
	}
	c.JSON(http.StatusOK, PemasokListResponse{
		Data: data,
		Meta: PageMeta{Page: res.Meta.Page, PerPage: res.Meta.PerPage, Total: res.Meta.Total},
	})
}

// GetKebutuhan handles GET /api/v1/pengadaan/kebutuhan
func (h *Handler) GetKebutuhan(c *gin.Context) {
	filter := domain.KebutuhanFilter{
		Dari:   c.Query("dari"),
		Sampai: c.Query("sampai"),
	}
	if v := c.Query("cadangan_persen"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"cadangan_persen": "must be a number"}})
			return
		}
		filter.CadanganPersen = f
	}
	if v := c.Query("kategori"); v != "" {
		filter.Kategori = &v
	}

	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.GetKebutuhan(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}
	// Convert application result to domain type for response
	domainRes := &domain.KebutuhanResult{
		Meta:  res.Meta,
		Items: res.Items,
	}
	c.JSON(http.StatusOK, toKebutuhanResponse(domainRes))
}
