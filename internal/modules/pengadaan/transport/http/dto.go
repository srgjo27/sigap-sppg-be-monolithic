package http

import (
	"encoding/json"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// ===== Pemasok DTOs =====

type CreatePemasokRequest struct {
	Nama       string `json:"nama" binding:"required"`
	JenisUsaha string `json:"jenis_usaha" binding:"required"`
	Kontak     string `json:"kontak"`
	NoHP       string `json:"no_hp"`
	Alamat     string `json:"alamat"`
	SPPGID     *int64 `json:"sppg_id,omitempty"` // only for admin
}

type UpdatePemasokRequest struct {
	Nama       *string `json:"nama,omitempty"`
	JenisUsaha *string `json:"jenis_usaha,omitempty"`
	Kontak     *string `json:"kontak,omitempty"`
	NoHP       *string `json:"no_hp,omitempty"`
	Alamat     *string `json:"alamat,omitempty"`
	Aktif      *bool   `json:"aktif,omitempty"`
	Alasan     *string `json:"alasan,omitempty"`  // required when aktif = false
	SPPGID     *int64  `json:"sppg_id,omitempty"` // only for admin
}

type PemasokResponse struct {
	ID         int64  `json:"id"`
	Nama       string `json:"nama"`
	JenisUsaha string `json:"jenis_usaha"`
	Kontak     string `json:"kontak"`
	NoHP       string `json:"no_hp"`
	Aktif      bool   `json:"aktif"`
}

type PemasokDetailResponse struct {
	ID         int64            `json:"id"`
	SPPGID     int64            `json:"sppg_id"`
	Nama       string           `json:"nama"`
	JenisUsaha string           `json:"jenis_usaha"`
	Kontak     string           `json:"kontak"`
	NoHP       string           `json:"no_hp"`
	Alamat     string           `json:"alamat"`
	Aktif      bool             `json:"aktif"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Kinerja    *KinerjaResponse `json:"kinerja,omitempty"`
}

type KinerjaResponse struct {
	PeriodeHari      int     `json:"periode_hari"`
	JumlahPO         int     `json:"jumlah_po"`
	TotalNilaiPO     string  `json:"total_nilai_po"`
	TotalPotongNota  string  `json:"total_potong_nota"`
	PersenDitolak    float64 `json:"persen_ditolak"`
	PersenTepatWaktu float64 `json:"persen_tepat_waktu"`
}

type PemasokListResponse struct {
	Data []PemasokResponse `json:"data"`
	Meta PageMeta          `json:"meta"`
}

type PageMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// ===== Kebutuhan DTOs =====

type KebutuhanItemResponse struct {
	BahanID             int64                      `json:"bahan_id"`
	Nama                string                     `json:"nama"`
	Kategori            string                     `json:"kategori"`
	Satuan              string                     `json:"satuan"`
	MudahRusak          bool                       `json:"mudah_rusak"`
	TanggalPakaiPertama string                     `json:"tanggal_pakai_pertama"`
	TotalGram           float64                    `json:"total_gram"`
	QtyKebutuhan        float64                    `json:"qty_kebutuhan"`
	QtySudahDipesan     float64                    `json:"qty_sudah_dipesan"`
	QtySisa             float64                    `json:"qty_sisa"`
	Rincian             []KebutuhanRincianResponse `json:"rincian"`
}

type KebutuhanRincianResponse struct {
	Tanggal string  `json:"tanggal"`
	MenuID  int64   `json:"menu_id"`
	Qty     float64 `json:"qty"`
}

type MenuBelumDisetujuiResponse struct {
	Tanggal  string `json:"tanggal"`
	MenuID   int64  `json:"menu_id"`
	NamaMenu string `json:"nama_menu"`
}

type POLintasRentangResponse struct {
	POID    int64   `json:"po_id"`
	NomorPO string  `json:"nomor_po"`
	Pemasok string  `json:"pemasok"`
	ItemIDs []int64 `json:"item_ids"`
}

type KebutuhanMetaResponse struct {
	Dari               string                       `json:"dari"`
	Sampai             string                       `json:"sampai"`
	CadanganPersen     float64                      `json:"cadangan_persen"`
	JumlahMenu         int                          `json:"jumlah_menu"`
	MenuBelumDisetujui []MenuBelumDisetujuiResponse `json:"menu_belum_disetujui"`
	POLintasRentang    []POLintasRentangResponse    `json:"po_lintas_rentang"`
}

type KebutuhanResponse struct {
	Meta  KebutuhanMetaResponse   `json:"meta"`
	Items []KebutuhanItemResponse `json:"items"`
}

// ===== Helpers =====

func toPemasokResponse(p *domain.Pemasok) PemasokResponse {
	return PemasokResponse{
		ID:         p.ID,
		Nama:       p.Nama,
		JenisUsaha: p.JenisUsaha,
		Kontak:     p.Kontak,
		NoHP:       p.NoHP,
		Aktif:      p.Aktif,
	}
}

func toPemasokDetailResponse(p *domain.PemasokDetail) PemasokDetailResponse {
	resp := PemasokDetailResponse{
		ID:         p.ID,
		SPPGID:     p.SPPGID,
		Nama:       p.Nama,
		JenisUsaha: p.JenisUsaha,
		Kontak:     p.Kontak,
		NoHP:       p.NoHP,
		Alamat:     p.Alamat,
		Aktif:      p.Aktif,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}
	if p.Kinerja != nil {
		resp.Kinerja = &KinerjaResponse{
			PeriodeHari:      p.Kinerja.PeriodeHari,
			JumlahPO:         p.Kinerja.JumlahPO,
			TotalNilaiPO:     p.Kinerja.TotalNilaiPO,
			TotalPotongNota:  p.Kinerja.TotalPotongNota,
			PersenDitolak:    p.Kinerja.PersenDitolak,
			PersenTepatWaktu: p.Kinerja.PersenTepatWaktu,
		}
	}
	return resp
}

func toKebutuhanItemResponse(item domain.KebutuhanItem) KebutuhanItemResponse {
	rincian := make([]KebutuhanRincianResponse, len(item.Rincian))
	for i, r := range item.Rincian {
		rincian[i] = KebutuhanRincianResponse{
			Tanggal: r.Tanggal,
			MenuID:  r.MenuID,
			Qty:     r.Qty,
		}
	}
	return KebutuhanItemResponse{
		BahanID:             item.BahanID,
		Nama:                item.Nama,
		Kategori:            item.Kategori,
		Satuan:              item.Satuan,
		MudahRusak:          item.MudahRusak,
		TanggalPakaiPertama: item.TanggalPakaiPertama,
		TotalGram:           item.TotalGram,
		QtyKebutuhan:        item.QtyKebutuhan,
		QtySudahDipesan:     item.QtySudahDipesan,
		QtySisa:             item.QtySisa,
		Rincian:             rincian,
	}
}

func toKebutuhanResponse(res *domain.KebutuhanResult) KebutuhanResponse {
	items := make([]KebutuhanItemResponse, len(res.Items))
	for i, item := range res.Items {
		items[i] = toKebutuhanItemResponse(item)
	}
	menuBelum := make([]MenuBelumDisetujuiResponse, len(res.Meta.MenuBelumDisetujui))
	for i, m := range res.Meta.MenuBelumDisetujui {
		menuBelum[i] = MenuBelumDisetujuiResponse{
			Tanggal:  m.Tanggal,
			MenuID:   m.MenuID,
			NamaMenu: m.NamaMenu,
		}
	}
	poLintas := make([]POLintasRentangResponse, len(res.Meta.POLintasRentang))
	for i, po := range res.Meta.POLintasRentang {
		poLintas[i] = POLintasRentangResponse{
			POID:    po.POID,
			NomorPO: po.NomorPO,
			Pemasok: po.Pemasok,
			ItemIDs: po.ItemIDs,
		}
	}
	return KebutuhanResponse{
		Meta: KebutuhanMetaResponse{
			Dari:               res.Meta.Dari,
			Sampai:             res.Meta.Sampai,
			CadanganPersen:     res.Meta.CadanganPersen,
			JumlahMenu:         res.Meta.JumlahMenu,
			MenuBelumDisetujui: menuBelum,
			POLintasRentang:    poLintas,
		},
		Items: items,
	}
}

// RawMessage unmarshal helper
func unmarshalRawMessage(raw json.RawMessage, target any) error {
	if raw == nil {
		return nil
	}
	return json.Unmarshal(raw, target)
}

// ===== Purchase Order DTOs =====

type POItemRequest struct {
	BahanID     int64   `json:"bahan_id" binding:"required"`
	QtyPesan    float64 `json:"qty_pesan" binding:"required"`
	HargaSatuan float64 `json:"harga_satuan" binding:"required"`
}

type CreatePORequest struct {
	PemasokID       int64           `json:"pemasok_id" binding:"required"`
	TglKirimDiminta string          `json:"tgl_kirim_diminta" binding:"required"` // YYYY-MM-DD
	SiklusID        *int64          `json:"siklus_id,omitempty"`
	MenuIDs         []int64         `json:"menu_ids,omitempty"`
	Catatan         string          `json:"catatan"`
	Items           []POItemRequest `json:"items" binding:"required,min=1,max=50"`
}

type POFilter struct {
	Status         []string `form:"status"`
	PemasokID      *int64   `form:"pemasok_id"`
	TglKirimDari   string   `form:"tgl_kirim_dari"`   // YYYY-MM-DD
	TglKirimSampai string   `form:"tgl_kirim_sampai"` // YYYY-MM-DD
	Q              string   `form:"q"`
	SPPGID         *int64   `form:"sppg_id"` // only for admin/pengawas
	Page           int      `form:"page,default=1"`
	PerPage        int      `form:"per_page,default=20"`
}

type POItemResponse struct {
	ID              int64   `json:"id"`
	BahanID         int64   `json:"bahan_id"`
	BahanNama       string  `json:"bahan_nama"`
	BahanSatuan     string  `json:"bahan_satuan"`
	BahanMudahRusak bool    `json:"bahan_mudah_rusak"`
	QtyPesan        float64 `json:"qty_pesan"`
	HargaSatuan     float64 `json:"harga_satuan"`
	Subtotal        float64 `json:"subtotal"`
	QtyDiterima     float64 `json:"qty_diterima"`
	QtyDitolak      float64 `json:"qty_ditolak"`
	QtyKurang       float64 `json:"qty_kurang"`
	Terpenuhi       bool    `json:"terpenuhi"`
	NilaiPotongNota float64 `json:"nilai_potong_nota"`
	NilaiTagihan    float64 `json:"nilai_tagihan"`
}

type POListItemResponse struct {
	ID                  int64      `json:"id"`
	NomorPO             string     `json:"nomor_po"`
	PemasokID           int64      `json:"pemasok_id"`
	PemasokNama         string     `json:"pemasok_nama"`
	TglPO               time.Time  `json:"tgl_po"`
	TglKirimDiminta     *time.Time `json:"tgl_kirim_diminta"`
	Status              string     `json:"status"`
	Total               float64    `json:"total"`
	Terlambat           bool       `json:"terlambat"`
	JumlahItem          int        `json:"jumlah_item"`
	JumlahItemTerpenuhi int        `json:"jumlah_item_terpenuhi"`
}

type POListResponse struct {
	Data []POListItemResponse `json:"data"`
	Meta PageMeta             `json:"meta"`
}

type PODetailRekapResponse struct {
	TotalPesanan      float64 `json:"total_pesanan"`
	TotalNilaiDikirim float64 `json:"total_nilai_dikirim"`
	TotalPotongNota   float64 `json:"total_potong_nota"`
	TotalTagihan      float64 `json:"total_tagihan"`
}

type PODetailPenerimaanResponse struct {
	ID            int64     `json:"id"`
	KodeLot       string    `json:"kode_lot"`
	POItemID      int64     `json:"po_item_id"`
	WaktuTerima   time.Time `json:"waktu_terima"`
	QtyDiterima   float64   `json:"qty_diterima"`
	QtyDitolak    float64   `json:"qty_ditolak"`
	Kondisi       string    `json:"kondisi"`
	DiperiksaOleh int64     `json:"diperiksa_oleh"`
}

type PODetailMenuResponse struct {
	ID       int64     `json:"id"`
	Tanggal  time.Time `json:"tanggal"`
	NamaMenu string    `json:"nama_menu"`
}

type PODetailResponse struct {
	ID              int64                        `json:"id"`
	SPPGID          int64                        `json:"sppg_id"`
	NomorPO         string                       `json:"nomor_po"`
	PemasokID       int64                        `json:"pemasok_id"`
	PemasokNama     string                       `json:"pemasok_nama"`
	SiklusID        *int64                       `json:"siklus_id,omitempty"`
	TglPO           time.Time                    `json:"tgl_po"`
	TglKirimDiminta *time.Time                   `json:"tgl_kirim_diminta"`
	Status          string                       `json:"status"`
	Catatan         string                       `json:"catatan"`
	DibuatOleh      int64                        `json:"dibuat_oleh"`
	DikirimOleh     *int64                       `json:"dikirim_oleh,omitempty"`
	DikirimAt       *time.Time                   `json:"dikirim_at,omitempty"`
	DitutupOleh     *int64                       `json:"ditutup_oleh,omitempty"`
	DitutupAt       *time.Time                   `json:"ditutup_at,omitempty"`
	AlasanTutup     string                       `json:"alasan_tutup"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
	Items           []POItemResponse             `json:"items"`
	RekapNilai      PODetailRekapResponse        `json:"rekap_nilai"`
	Penerimaan      []PODetailPenerimaanResponse `json:"penerimaan"`
	Menu            []PODetailMenuResponse       `json:"menu"`
	Warnings        []string                     `json:"warnings,omitempty"`
}

// ===== PO update & transition DTOs =====

type UpdatePOItemRequest struct {
	BahanID     int64   `json:"bahan_id"`
	QtyPesan    float64 `json:"qty_pesan"`
	HargaSatuan float64 `json:"harga_satuan"`
}

type POTransitionResponse struct {
	PODetailResponse
	Warnings           []string         `json:"warnings,omitempty"`
	ItemTidakTerpenuhi []POItemResponse `json:"item_tidak_terpenuhi,omitempty"`
}

func toPOTransitionResponse(detail *domain.PODetail, warnings []string, tidakTerpenuhi []domain.PODetailItem) POTransitionResponse {
	resp := POTransitionResponse{PODetailResponse: toPODetailResponse(detail)}
	if warnings != nil {
		resp.Warnings = warnings
	}
	if tidakTerpenuhi != nil {
		items := make([]POItemResponse, len(tidakTerpenuhi))
		for i, it := range tidakTerpenuhi {
			items[i] = toPOItemResponse(it)
		}
		resp.ItemTidakTerpenuhi = items
	}
	return resp
}

// Helpers for PO

func toPOItemResponse(item domain.PODetailItem) POItemResponse {
	return POItemResponse{
		ID:              item.ID,
		BahanID:         item.BahanID,
		BahanNama:       item.BahanNama,
		BahanSatuan:     item.BahanSatuan,
		BahanMudahRusak: item.BahanMudahRusak,
		QtyPesan:        item.QtyPesan,
		HargaSatuan:     item.HargaSatuan,
		Subtotal:        item.Subtotal,
		QtyDiterima:     item.QtyDiterima,
		QtyDitolak:      item.QtyDitolak,
		QtyKurang:       item.QtyKurang,
		Terpenuhi:       item.Terpenuhi,
		NilaiPotongNota: item.NilaiPotongNota,
		NilaiTagihan:    item.NilaiTagihan,
	}
}

func toPOListItemResponse(item domain.POListItem) POListItemResponse {
	return POListItemResponse{
		ID:                  item.ID,
		NomorPO:             item.NomorPO,
		PemasokID:           item.PemasokID,
		PemasokNama:         item.PemasokNama,
		TglPO:               item.TglPO,
		TglKirimDiminta:     item.TglKirimDiminta,
		Status:              item.Status,
		Total:               item.Total,
		Terlambat:           item.Terlambat,
		JumlahItem:          item.JumlahItem,
		JumlahItemTerpenuhi: item.JumlahItemTerpenuhi,
	}
}

// toPOCreateResponse creates a response for PO creation.
func toPOCreateResponse(res *application.POCreateResult) PODetailResponse {
	items := make([]POItemResponse, len(res.Items))
	for i, item := range res.Items {
		items[i] = POItemResponse{
			ID:              item.ID,
			BahanID:         item.BahanID,
			BahanNama:       "", // Will be filled if needed
			BahanSatuan:     "",
			BahanMudahRusak: false,
			QtyPesan:        item.QtyPesan,
			HargaSatuan:     item.HargaSatuan,
			Subtotal:        item.Subtotal,
			QtyDiterima:     0,
			QtyDitolak:      0,
			QtyKurang:       item.QtyPesan,
			Terpenuhi:       false,
			NilaiPotongNota: 0,
			NilaiTagihan:    0,
		}
	}
	return PODetailResponse{
		ID:              res.PO.ID,
		SPPGID:          res.PO.SPPGID,
		NomorPO:         res.PO.NomorPO,
		PemasokID:       res.PO.PemasokID,
		PemasokNama:     "", // Would need to fetch pemasok name
		SiklusID:        res.PO.SiklusID,
		TglPO:           res.PO.TglPO,
		TglKirimDiminta: res.PO.TglKirimDiminta,
		Status:          res.PO.Status,
		Catatan:         derefString(res.PO.Catatan),
		DibuatOleh:      res.PO.DibuatOleh,
		DikirimOleh:     res.PO.DikirimOleh,
		DikirimAt:       res.PO.DikirimAt,
		DitutupOleh:     res.PO.DitutupOleh,
		DitutupAt:       res.PO.DitutupAt,
		AlasanTutup:     derefString(res.PO.AlasanTutup),
		CreatedAt:       res.PO.CreatedAt,
		UpdatedAt:       res.PO.UpdatedAt,
		Items:           items,
		Warnings:        res.Warnings,
	}
}

func toPOListResponse(res *application.POListResult) POListResponse {
	data := make([]POListItemResponse, len(res.Data))
	for i, item := range res.Data {
		data[i] = toPOListItemResponse(item)
	}
	return POListResponse{
		Data: data,
		Meta: PageMeta{Page: res.Meta.Page, PerPage: res.Meta.PerPage, Total: res.Meta.Total},
	}
}

func toPODetailResponse(detail *domain.PODetail) PODetailResponse {
	items := make([]POItemResponse, len(detail.Items))
	for i, item := range detail.Items {
		items[i] = toPOItemResponse(item)
	}
	penerimaan := make([]PODetailPenerimaanResponse, len(detail.Penerimaan))
	for i, p := range detail.Penerimaan {
		penerimaan[i] = PODetailPenerimaanResponse{
			ID:            p.ID,
			KodeLot:       p.KodeLot,
			POItemID:      p.POItemID,
			WaktuTerima:   p.WaktuTerima,
			QtyDiterima:   p.QtyDiterima,
			QtyDitolak:    p.QtyDitolak,
			Kondisi:       p.Kondisi,
			DiperiksaOleh: p.DiperiksaOleh,
		}
	}
	menu := make([]PODetailMenuResponse, len(detail.Menu))
	for i, m := range detail.Menu {
		menu[i] = PODetailMenuResponse{
			ID:       m.ID,
			Tanggal:  m.Tanggal,
			NamaMenu: m.NamaMenu,
		}
	}
	return PODetailResponse{
		ID:              detail.ID,
		SPPGID:          detail.SPPGID,
		NomorPO:         detail.NomorPO,
		PemasokID:       detail.PemasokID,
		PemasokNama:     detail.PemasokNama,
		SiklusID:        detail.SiklusID,
		TglPO:           detail.TglPO,
		TglKirimDiminta: detail.TglKirimDiminta,
		Status:          detail.Status,
		Catatan:         derefString(detail.Catatan),
		DibuatOleh:      detail.DibuatOleh,
		DikirimOleh:     detail.DikirimOleh,
		DikirimAt:       detail.DikirimAt,
		DitutupOleh:     detail.DitutupOleh,
		DitutupAt:       detail.DitutupAt,
		AlasanTutup:     derefString(detail.AlasanTutup),
		CreatedAt:       detail.CreatedAt,
		UpdatedAt:       detail.UpdatedAt,
		Items:           items,
		RekapNilai: PODetailRekapResponse{
			TotalPesanan:      detail.RekapNilai.TotalPesanan,
			TotalNilaiDikirim: detail.RekapNilai.TotalNilaiDikirim,
			TotalPotongNota:   detail.RekapNilai.TotalPotongNota,
			TotalTagihan:      detail.RekapNilai.TotalTagihan,
		},
		Penerimaan: penerimaan,
		Menu:       menu,
	}
}

// derefString returns the string value or empty string if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
