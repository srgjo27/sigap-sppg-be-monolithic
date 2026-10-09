package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
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

// CreateBahan handles POST /bahan (MVP-002.1).
func (h *Handler) CreateBahan(c *gin.Context) {
	var req CreateBahanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	mudah := false
	if req.MudahRusak != nil {
		mudah = *req.MudahRusak
	}
	res, err := h.svc.CreateBahan(c.Request.Context(), claims, domain.CreateBahanInput{
		Nama: req.Nama, Kategori: req.Kategori, Satuan: req.Satuan,
		GramPerSatuan: req.GramPerSatuan, MudahRusak: mudah, SuhuSimpanMaks: req.SuhuSimpanMaks,
	}, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toBahanResponse(res))
}

// UpdateBahan handles PATCH /bahan/:id (MVP-002.1).
func (h *Handler) UpdateBahan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrBahanNotFound)
		return
	}
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	var req UpdateBahanRequest
	if v, ok := raw["nama"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"nama": "must be a string"}})
			return
		}
		req.Nama = &s
	}
	if v, ok := raw["kategori"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"kategori": "must be a string"}})
			return
		}
		req.Kategori = &s
	}
	if v, ok := raw["satuan"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"satuan": "must be a string"}})
			return
		}
		req.Satuan = &s
	}
	if v, ok := raw["gram_per_satuan"]; ok {
		var f float64
		if err := json.Unmarshal(v, &f); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"gram_per_satuan": "must be a number"}})
			return
		}
		req.GramPerSatuan = &f
	}
	if v, ok := raw["mudah_rusak"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"mudah_rusak": "must be a boolean"}})
			return
		}
		req.MudahRusak = &b
	}
	if v, ok := raw["suhu_simpan_maks"]; ok {
		if string(v) == "null" {
			req.rawSuhuNull = true
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"suhu_simpan_maks": "must be a number"}})
				return
			}
			req.SuhuSimpanMaks = &f
		}
	}
	if v, ok := raw["aktif"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"aktif": "must be a boolean"}})
			return
		}
		req.Aktif = &b
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.UpdateBahan(c.Request.Context(), claims, id, domain.UpdateBahanInput{
		Nama: req.Nama, Kategori: req.Kategori, Satuan: req.Satuan,
		GramPerSatuan: req.GramPerSatuan, MudahRusak: req.MudahRusak,
		SuhuSimpanMaks: req.SuhuSimpanMaks, Aktif: req.Aktif, ClearSuhu: req.rawSuhuNull,
	}, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toBahanResponse(res))
}

// ListBahan handles GET /bahan?q=&kategori=&aktif= (MVP-002.1).
func (h *Handler) ListBahan(c *gin.Context) {
	claims, _ := authhttp.CurrentUser(c)
	filter := domain.BahanFilter{}
	if v := c.Query("q"); v != "" {
		filter.Q = &v
	}
	if v := c.Query("kategori"); v != "" {
		filter.Kategori = &v
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
	items, total, err := h.svc.ListBahan(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}
	filter.Normalize()
	data := make([]BahanResponse, 0, len(items))
	for i := range items {
		data = append(data, toBahanResponse(&items[i]))
	}
	c.JSON(http.StatusOK, BahanListResponse{Data: data, Meta: PageMeta{Page: filter.Page, PerPage: filter.Limit, Total: total}})
}

// CreateMenu handles POST /menus (MVP-002.2).
func (h *Handler) CreateMenu(c *gin.Context) {
	var req CreateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	tanggal, err := time.Parse("2006-01-02", req.Tanggal)
	if err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"tanggal": "use YYYY-MM-DD"}})
		return
	}
	if req.EnergiKkal == nil || req.ProteinG == nil || req.KarbohidratG == nil || req.LemakG == nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"gizi": "energi_kkal, protein_g, karbohidrat_g, lemak_g are required"}})
		return
	}
	items := make([]domain.MenuBahanInput, 0, len(req.Bahan))
	for _, b := range req.Bahan {
		if b.GramPerPorsi == nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"bahan": "gram_per_porsi is required"}})
			return
		}
		items = append(items, domain.MenuBahanInput{BahanID: b.BahanID, GramPerPorsi: *b.GramPerPorsi})
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.CreateMenu(c.Request.Context(), claims, domain.CreateMenuInput{
		Tanggal: tanggal, NamaMenu: req.NamaMenu,
		EnergiKkal: *req.EnergiKkal, ProteinG: *req.ProteinG, KarbohidratG: *req.KarbohidratG, LemakG: *req.LemakG,
		TargetPorsi: req.TargetPorsi, Catatan: req.Catatan, Items: items,
	}, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toCreateMenuResponse(res))
}
