package http

import (
	"time"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

// CreateBahanRequest is POST /api/v1/bahan input.
type CreateBahanRequest struct {
	Nama           string   `json:"nama" binding:"required"`
	Kategori       string   `json:"kategori" binding:"required"`
	Satuan         string   `json:"satuan" binding:"required"`
	GramPerSatuan  *float64 `json:"gram_per_satuan"`
	MudahRusak     *bool    `json:"mudah_rusak"`
	SuhuSimpanMaks *float64 `json:"suhu_simpan_maks"`
}

// UpdateBahanRequest is PATCH /api/v1/bahan/:id input. Pointers detect omission;
// send suhu_simpan_maks as null to clear.
type UpdateBahanRequest struct {
	Nama           *string  `json:"nama"`
	Kategori       *string  `json:"kategori"`
	Satuan         *string  `json:"satuan"`
	GramPerSatuan  *float64 `json:"gram_per_satuan"`
	MudahRusak     *bool    `json:"mudah_rusak"`
	SuhuSimpanMaks *float64 `json:"suhu_simpan_maks"`
	Aktif          *bool    `json:"aktif"`
	rawSuhuNull    bool
}

// BahanResponse is the public bahan shape.
type BahanResponse struct {
	ID             int64     `json:"id"`
	Nama           string    `json:"nama"`
	Kategori       string    `json:"kategori"`
	Satuan         string    `json:"satuan"`
	GramPerSatuan  float64   `json:"gram_per_satuan"`
	MudahRusak     bool      `json:"mudah_rusak"`
	SuhuSimpanMaks *float64  `json:"suhu_simpan_maks,omitempty"`
	Aktif          bool      `json:"aktif"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BahanListResponse wraps paginated bahan rows.
type BahanListResponse struct {
	Data []BahanResponse `json:"data"`
	Meta PageMeta        `json:"meta"`
}

// PageMeta describes pagination.
type PageMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// CreateMenuBahanRequest is one composition item.
type CreateMenuBahanRequest struct {
	BahanID      int64    `json:"bahan_id" binding:"required"`
	GramPerPorsi *float64 `json:"gram_per_porsi" binding:"required"`
}

// CreateMenuRequest is POST /api/v1/menus input.
type CreateMenuRequest struct {
	Tanggal      string                   `json:"tanggal" binding:"required"`
	NamaMenu     string                   `json:"nama_menu" binding:"required"`
	EnergiKkal   *float64                 `json:"energi_kkal" binding:"required"`
	ProteinG     *float64                 `json:"protein_g" binding:"required"`
	KarbohidratG *float64                 `json:"karbohidrat_g" binding:"required"`
	LemakG       *float64                 `json:"lemak_g" binding:"required"`
	TargetPorsi  *int                     `json:"target_porsi"`
	Catatan      *string                  `json:"catatan"`
	Bahan        []CreateMenuBahanRequest `json:"bahan" binding:"required"`
}

// MenuGiziResponse groups nutrition per portion.
type MenuGiziResponse struct {
	EnergiKkal   float64 `json:"energi_kkal"`
	ProteinG     float64 `json:"protein_g"`
	KarbohidratG float64 `json:"karbohidrat_g"`
	LemakG       float64 `json:"lemak_g"`
}

// MenuBahanResponse is one composition item in responses.
type MenuBahanResponse struct {
	BahanID      int64   `json:"bahan_id"`
	Nama         string  `json:"nama"`
	GramPerPorsi float64 `json:"gram_per_porsi"`
}

// CreateMenuResponse is the MVP-002.2 success payload.
type CreateMenuResponse struct {
	ID          int64               `json:"id"`
	Tanggal     string              `json:"tanggal"`
	NamaMenu    string              `json:"nama_menu"`
	Gizi        MenuGiziResponse    `json:"gizi"`
	TargetPorsi int                 `json:"target_porsi"`
	Status      string              `json:"status"`
	Bahan       []MenuBahanResponse `json:"bahan"`
	DibuatOleh  int64               `json:"dibuat_oleh"`
	CreatedAt   time.Time           `json:"created_at"`
	Warnings    []string            `json:"warnings"`
}

func toBahanResponse(b *domain.Bahan) BahanResponse {
	return BahanResponse{
		ID: b.ID, Nama: b.Nama, Kategori: b.Kategori, Satuan: b.Satuan,
		GramPerSatuan: b.GramPerSatuan, MudahRusak: b.MudahRusak,
		SuhuSimpanMaks: b.SuhuSimpanMaks, Aktif: b.Aktif,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

func toCreateMenuResponse(res *application.CreateMenuResult) CreateMenuResponse {
	items := make([]MenuBahanResponse, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, MenuBahanResponse{BahanID: it.BahanID, Nama: it.BahanNama, GramPerPorsi: it.GramPerPorsi})
	}
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return CreateMenuResponse{
		ID: res.Menu.ID, Tanggal: res.Menu.Tanggal.Format("2006-01-02"), NamaMenu: res.Menu.NamaMenu,
		Gizi:        MenuGiziResponse{EnergiKkal: res.Menu.EnergiKkal, ProteinG: res.Menu.ProteinG, KarbohidratG: res.Menu.KarbohidratG, LemakG: res.Menu.LemakG},
		TargetPorsi: res.Menu.TargetPorsi, Status: res.Menu.Status, Bahan: items,
		DibuatOleh: res.Menu.DibuatOleh, CreatedAt: res.Menu.CreatedAt, Warnings: warnings,
	}
}
