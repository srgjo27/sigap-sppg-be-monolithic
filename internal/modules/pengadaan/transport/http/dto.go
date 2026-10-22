package http

import (
	"encoding/json"
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// ===== Pemasok DTOs =====

type CreatePemasokRequest struct {
	Nama       string `json:"nama" binding:"required"`
	JenisUsaha string `json:"jenis_usaha" binding:"required"`
	Kontak     string `json:"kontak"`
	NoHP       string `json:"no_hp" binding:"required"`
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
