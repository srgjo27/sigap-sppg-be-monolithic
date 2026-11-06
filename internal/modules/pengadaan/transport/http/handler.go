package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
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

// CreatePurchaseOrder handles POST /api/v1/purchase-orders
func (h *Handler) CreatePurchaseOrder(c *gin.Context) {
	var req CreatePORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	items := make([]domain.POItemInput, len(req.Items))
	for i, item := range req.Items {
		items[i] = domain.POItemInput{
			BahanID:     item.BahanID,
			QtyPesan:    item.QtyPesan,
			HargaSatuan: item.HargaSatuan,
		}
	}
	in := domain.CreatePOInput{
		PemasokID:       req.PemasokID,
		TglKirimDiminta: req.TglKirimDiminta,
		SiklusID:        req.SiklusID,
		MenuIDs:         req.MenuIDs,
		Catatan:         req.Catatan,
		Items:           items,
	}
	res, err := h.svc.CreatePurchaseOrder(c.Request.Context(), claims, in, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPOCreateResponse(res))
}

// ListPurchaseOrders handles GET /api/v1/purchase-orders
func (h *Handler) ListPurchaseOrders(c *gin.Context) {
	claims, _ := authhttp.CurrentUser(c)
	filter := domain.POFilter{}
	// Parse status (comma-separated)
	if v := c.Query("status"); v != "" {
		filter.Status = strings.Split(v, ",")
	}
	if v := c.Query("pemasok_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.PemasokID = &id
		}
	}
	if v := c.Query("tgl_kirim_dari"); v != "" {
		filter.TglKirimDari = &v
	}
	if v := c.Query("tgl_kirim_sampai"); v != "" {
		filter.TglKirimSampai = &v
	}
	if v := c.Query("q"); v != "" {
		filter.Q = &v
	}
	if v := c.Query("sppg_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.SPPGID = &id
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
	res, err := h.svc.ListPurchaseOrders(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPOListResponse(res))
}

// GetPurchaseOrder handles GET /api/v1/purchase-orders/{id}
func (h *Handler) GetPurchaseOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPONotFound)
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.GetPurchaseOrder(c.Request.Context(), claims, id)
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPODetailResponse(res.Detail))
}

// UpdatePurchaseOrder handles PATCH /api/v1/purchase-orders/{id}.
// Field presence is detected on the raw JSON so PATCH keeps untouched fields.
func (h *Handler) UpdatePurchaseOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPONotFound)
		return
	}
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}

	var in domain.UpdatePOInput
	if v, ok := raw["pemasok_id"]; ok {
		if string(v) == "null" {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"pemasok_id": "cannot be null"}})
			return
		}
		if err := unmarshalRawMessage(v, &in.PemasokID); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"pemasok_id": "must be an integer"}})
			return
		}
	}
	if v, ok := raw["tgl_kirim_diminta"]; ok {
		if string(v) == "null" {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"tgl_kirim_diminta": "cannot be null"}})
			return
		}
		var s string
		if err := unmarshalRawMessage(v, &s); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"tgl_kirim_diminta": "must be a string"}})
			return
		}
		in.TglKirimDiminta = &s
	}
	if v, ok := raw["siklus_id"]; ok {
		if string(v) != "null" {
			if err := unmarshalRawMessage(v, &in.SiklusID); err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"siklus_id": "must be an integer"}})
				return
			}
		}
		in.SiklusIDSet = true
	}
	if v, ok := raw["menu_ids"]; ok {
		if string(v) != "null" {
			var ids []int64
			if err := unmarshalRawMessage(v, &ids); err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"menu_ids": "must be an array of integers"}})
				return
			}
			in.MenuIDs = ids
		}
		in.MenuIDsSet = true
	}
	if v, ok := raw["catatan"]; ok {
		var s string
		if string(v) == "null" {
			s = ""
		} else if err := unmarshalRawMessage(v, &s); err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"catatan": "must be a string"}})
			return
		}
		in.Catatan = &s
	}
	if v, ok := raw["items"]; ok {
		if string(v) != "null" {
			var reqs []UpdatePOItemRequest
			if err := unmarshalRawMessage(v, &reqs); err != nil {
				MapError(c, &domain.ValidationError{Fields: map[string]string{"items": "must be an array of items"}})
				return
			}
			items := make([]domain.POItemInput, len(reqs))
			for i, r := range reqs {
				items[i] = domain.POItemInput{BahanID: r.BahanID, QtyPesan: r.QtyPesan, HargaSatuan: r.HargaSatuan}
			}
			in.Items = items
		}
		in.ItemsSet = true
	}

	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.UpdatePurchaseOrder(c.Request.Context(), claims, id, in, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPOTransitionResponse(res.Detail, res.Warnings, nil))
}

// DeletePurchaseOrder handles DELETE /api/v1/purchase-orders/{id}.
func (h *Handler) DeletePurchaseOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPONotFound)
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	if err := h.svc.DeletePurchaseOrder(c.Request.Context(), claims, id, clientIP(c)); err != nil {
		MapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// KirimPurchaseOrder handles POST /api/v1/purchase-orders/{id}/kirim.
func (h *Handler) KirimPurchaseOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPONotFound)
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.KirimPurchaseOrder(c.Request.Context(), claims, id, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPOTransitionResponse(res.Detail, nil, nil))
}

type POAlasanRequest struct {
	Alasan string `json:"alasan" binding:"required"`
}

// BatalPurchaseOrder handles POST /api/v1/purchase-orders/{id}/batal.
func (h *Handler) BatalPurchaseOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPONotFound)
		return
	}
	var req POAlasanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"alasan": "required"}})
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.BatalPurchaseOrder(c.Request.Context(), claims, id, req.Alasan, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPOTransitionResponse(res.Detail, nil, nil))
}

// TutupPurchaseOrder handles POST /api/v1/purchase-orders/{id}/tutup.
func (h *Handler) TutupPurchaseOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPONotFound)
		return
	}
	var req POAlasanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"alasan": "required"}})
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.TutupPurchaseOrder(c.Request.Context(), claims, id, req.Alasan, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPOTransitionResponse(res.Detail, nil, res.ItemTidakTerpenuhi))
}

// CreatePenerimaan handles POST /api/v1/purchase-orders/{id}/penerimaan.
func (h *Handler) CreatePenerimaan(c *gin.Context) {
	poID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrPONotFound)
		return
	}
	var req CreatePenerimaanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	clientRef := c.GetHeader("Idempotency-Key")
	if clientRef == "" {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"idempotency_key": "required"}})
		return
	}

	items := make([]domain.PenerimaanItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = domain.PenerimaanItem{
			POItemID:        item.POItemID,
			QtyDiterima:     item.QtyDiterima,
			QtyDitolak:      item.QtyDitolak,
			AlasanTolak:     item.AlasanTolak,
			SuhuTerima:      item.SuhuTerima,
			Kondisi:         item.Kondisi,
			TglKedaluwarsa:  item.TglKedaluwarsa,
			FotoBerkasID:    item.FotoBerkasID,
			LolosSuhu:       item.LolosSuhu,
			Catatan:         item.Catatan,
			NilaiPotongNota: item.NilaiPotongNota,
		}
	}

	claims, _ := authhttp.CurrentUser(c)
	in := domain.CreatePenerimaanInput{
		Items:     items,
		ClientRef: clientRef,
	}
	res, err := h.svc.CreatePenerimaan(c.Request.Context(), claims, poID, in, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}

	lots := make([]PenerimaanDetailResponse, len(res.Lots))
	for i, lot := range res.Lots {
		lots[i] = toPenerimaanDetailResponse(&domain.PenerimaanDetail{
			ID:              lot.ID,
			POItemID:        lot.POItemID,
			KodeLot:         lot.KodeLot,
			WaktuTerima:     lot.WaktuTerima,
			QtyDiterima:     lot.QtyDiterima,
			QtyDitolak:      lot.QtyDitolak,
			AlasanTolak:     lot.AlasanTolak,
			SuhuTerima:      lot.SuhuTerima,
			Kondisi:         lot.Kondisi,
			TglKedaluwarsa:  lot.TglKedaluwarsa,
			FotoURL:         lot.FotoURL,
			DiperiksaOleh:   lot.DiperiksaOleh,
			NilaiPotongNota: lot.NilaiPotongNota,
			SPPGID:          lot.SPPGID,
			ClientRef:       lot.ClientRef,
			Catatan:         lot.Catatan,
			CreatedAt:       lot.CreatedAt,
			LolosSuhu:       lot.LolosSuhu,
		})
	}
	c.JSON(http.StatusCreated, gin.H{
		"po": map[string]any{
			"id":       res.PO.ID,
			"nomor_po": res.PO.NomorPO,
			"status":   res.PO.Status,
			"rekap_nilai": map[string]any{
				"total_pesanan":       res.PO.Total,
				"total_nilai_dikirim": 0,
				"total_potong_nota":   0,
				"total_tagihan":       0,
			},
		},
		"lots":     lots,
		"warnings": res.Warnings,
	})
}

// ListPenerimaan handles GET /api/v1/penerimaan.
func (h *Handler) ListPenerimaan(c *gin.Context) {
	claims, _ := authhttp.CurrentUser(c)
	filter := domain.PenerimaanFilter{}
	if v := c.Query("dari"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"dari": "use YYYY-MM-DD"}})
			return
		}
		filter.Dari = &t
	}
	if v := c.Query("sampai"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"sampai": "use YYYY-MM-DD"}})
			return
		}
		filter.Sampai = &t
	}
	if v := c.Query("bahan_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.BahanID = &id
		}
	}
	if v := c.Query("pemasok_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.PemasokID = &id
		}
	}
	if v := c.Query("po_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.POID = &id
		}
	}
	if v := c.Query("kondisi"); v != "" {
		filter.Kondisi = &v
	}
	if v := c.Query("kode_lot"); v != "" {
		filter.KodeLot = &v
	}
	if v := c.Query("tersedia"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			MapError(c, &domain.ValidationError{Fields: map[string]string{"tersedia": "must be a boolean"}})
			return
		}
		filter.Tersedia = &parsed
	}
	if v := c.Query("sppg_id"); v != "" {
		if claims.Role == authdomain.RoleAdmin || claims.Role == authdomain.RolePengawas {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				filter.SPPGID = &id
			}
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

	// Default: last 7 days.
	if filter.Dari == nil && filter.Sampai == nil {
		now := time.Now()
		dari := now.AddDate(0, 0, -7)
		filter.Dari = &dari
		filter.Sampai = &now
	}

	res, err := h.svc.ListPenerimaan(c.Request.Context(), claims, filter)
	if err != nil {
		MapError(c, err)
		return
	}

	// petugas_dapur: hide price fields.
	isPetugasDapur := claims.Role == authdomain.RolePetugasDapur
	data := make([]PenerimaanListItemResponse, len(res.Data))
	for i, item := range res.Data {
		resp := toPenerimaanListItemResponse(item)
		if isPetugasDapur {
			resp.NilaiPotongNota = 0
		}
		data[i] = resp
	}
	c.JSON(http.StatusOK, PenerimaanListResponse{
		Data: data,
		Meta: PageMeta{Page: res.Meta.Page, PerPage: res.Meta.PerPage, Total: res.Meta.Total},
	})
}

// GetPenerimaan handles GET /api/v1/penerimaan/{id}.
func (h *Handler) GetPenerimaan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrLotNotFound)
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.GetPenerimaan(c.Request.Context(), claims, id)
	if err != nil {
		MapError(c, err)
		return
	}
	resp := toPenerimaanDetailResponse(res.Detail)
	if claims.Role == authdomain.RolePetugasDapur {
		resp.NilaiPotongNota = 0
	}
	c.JSON(http.StatusOK, resp)
}

// KoreksiPenerimaan handles POST /api/v1/penerimaan/{id}/koreksi.
func (h *Handler) KoreksiPenerimaan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		MapError(c, domain.ErrLotNotFound)
		return
	}
	var req KoreksiPenerimaanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		MapError(c, &domain.ValidationError{Fields: map[string]string{"body": "invalid JSON"}})
		return
	}
	claims, _ := authhttp.CurrentUser(c)
	res, err := h.svc.KoreksiPenerimaan(c.Request.Context(), claims, id, domain.KoreksiPenerimaanInput{
		Alasan:         req.Alasan,
		QtyDiterima:    req.QtyDiterima,
		QtyDitolak:     req.QtyDitolak,
		AlasanTolak:    req.AlasanTolak,
		Kondisi:        req.Kondisi,
		TglKedaluwarsa: req.TglKedaluwarsa,
		Catatan:        req.Catatan,
	}, clientIP(c))
	if err != nil {
		MapError(c, err)
		return
	}
	c.JSON(http.StatusOK, toKoreksiPenerimaanResponse(res.Detail, res.PO))
}
