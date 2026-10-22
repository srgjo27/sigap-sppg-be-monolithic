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
// GramPerPorsi is omitted when zero so pic_sekolah views stay gram-free.
type MenuBahanResponse struct {
	BahanID      int64   `json:"bahan_id"`
	Nama         string  `json:"nama"`
	GramPerPorsi float64 `json:"gram_per_porsi,omitempty"`
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

// MenuListItemResponse is one row in GET /menus.
type MenuListItemResponse struct {
	ID          int64            `json:"id"`
	Tanggal     string           `json:"tanggal"`
	NamaMenu    string           `json:"nama_menu"`
	Gizi        MenuGiziResponse `json:"gizi"`
	TargetPorsi int              `json:"target_porsi"`
	Status      string           `json:"status"`
}

// MenuListMeta carries range info plus empty dates.
type MenuListMeta struct {
	Dari          string   `json:"dari"`
	Sampai        string   `json:"sampai"`
	Total         int      `json:"total"`
	TanggalKosong []string `json:"tanggal_kosong"`
}

// MenuListResponse is the MVP-002.3 list payload.
type MenuListResponse struct {
	Data []MenuListItemResponse `json:"data"`
	Meta MenuListMeta           `json:"meta"`
}

// MenuDetailResponse is the MVP-002.3 detail payload.
type MenuDetailResponse struct {
	ID            int64               `json:"id"`
	Tanggal       string              `json:"tanggal"`
	NamaMenu      string              `json:"nama_menu"`
	Gizi          MenuGiziResponse    `json:"gizi"`
	TargetPorsi   int                 `json:"target_porsi"`
	Status        string              `json:"status"`
	Bahan         []MenuBahanResponse `json:"bahan"`
	DibuatOleh    int64               `json:"dibuat_oleh"`
	DisetujuiOleh *int64              `json:"disetujui_oleh,omitempty"`
	DisetujuiAt   *time.Time          `json:"disetujui_at,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
}

// UpdateMenuBahanRequest is one replacement composition item.
type UpdateMenuBahanRequest struct {
	BahanID      int64    `json:"bahan_id"`
	GramPerPorsi *float64 `json:"gram_per_porsi"`
}

// UpdateMenuResponse reuses the create shape (updated menu + warnings).
type UpdateMenuResponse = CreateMenuResponse

func toMenuListResponse(res *application.MenuListResult, dari, sampai string) MenuListResponse {
	data := make([]MenuListItemResponse, 0, len(res.Items))
	for _, m := range res.Items {
		data = append(data, MenuListItemResponse{
			ID: m.ID, Tanggal: m.Tanggal.Format("2006-01-02"), NamaMenu: m.NamaMenu,
			Gizi:        MenuGiziResponse{EnergiKkal: m.EnergiKkal, ProteinG: m.ProteinG, KarbohidratG: m.KarbohidratG, LemakG: m.LemakG},
			TargetPorsi: m.TargetPorsi, Status: m.Status,
		})
	}
	missing := res.Missing
	if missing == nil {
		missing = []string{}
	}
	return MenuListResponse{Data: data, Meta: MenuListMeta{Dari: dari, Sampai: sampai, Total: res.Total, TanggalKosong: missing}}
}

func toMenuDetailResponse(det *application.MenuDetail) MenuDetailResponse {
	items := make([]MenuBahanResponse, 0, len(det.Items))
	for _, it := range det.Items {
		items = append(items, MenuBahanResponse{BahanID: it.BahanID, Nama: it.BahanNama, GramPerPorsi: it.GramPerPorsi})
	}
	return MenuDetailResponse{
		ID: det.Menu.ID, Tanggal: det.Menu.Tanggal.Format("2006-01-02"), NamaMenu: det.Menu.NamaMenu,
		Gizi:        MenuGiziResponse{EnergiKkal: det.Menu.EnergiKkal, ProteinG: det.Menu.ProteinG, KarbohidratG: det.Menu.KarbohidratG, LemakG: det.Menu.LemakG},
		TargetPorsi: det.Menu.TargetPorsi, Status: det.Menu.Status, Bahan: items,
		DibuatOleh: det.Menu.DibuatOleh, DisetujuiOleh: det.Menu.DisetujuiOleh, DisetujuiAt: det.Menu.DisetujuiAt,
		CreatedAt: det.Menu.CreatedAt,
	}
}

func toUpdateMenuResponse(res *application.UpdateMenuResult) UpdateMenuResponse {
	items := make([]MenuBahanResponse, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, MenuBahanResponse{BahanID: it.BahanID, Nama: it.BahanNama, GramPerPorsi: it.GramPerPorsi})
	}
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return UpdateMenuResponse{
		ID: res.Menu.ID, Tanggal: res.Menu.Tanggal.Format("2006-01-02"), NamaMenu: res.Menu.NamaMenu,
		Gizi:        MenuGiziResponse{EnergiKkal: res.Menu.EnergiKkal, ProteinG: res.Menu.ProteinG, KarbohidratG: res.Menu.KarbohidratG, LemakG: res.Menu.LemakG},
		TargetPorsi: res.Menu.TargetPorsi, Status: res.Menu.Status, Bahan: items,
		DibuatOleh: res.Menu.DibuatOleh, CreatedAt: res.Menu.CreatedAt, Warnings: warnings,
	}
}

// RevertRequest is POST /menus/{id}/revert input (alasan required).
type RevertRequest struct {
	Alasan string `json:"alasan"`
}

// ApprovalResponse is the MVP-002.5 success payload (menu with new status).
type ApprovalResponse struct {
	ID            int64               `json:"id"`
	Tanggal       string              `json:"tanggal"`
	NamaMenu      string              `json:"nama_menu"`
	Gizi          MenuGiziResponse    `json:"gizi"`
	TargetPorsi   int                 `json:"target_porsi"`
	Status        string              `json:"status"`
	Bahan         []MenuBahanResponse `json:"bahan"`
	DibuatOleh    int64               `json:"dibuat_oleh"`
	DisetujuiOleh *int64              `json:"disetujui_oleh,omitempty"`
	DisetujuiAt   *time.Time          `json:"disetujui_at,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
}

// KebutuhanMenuRef summarizes the menu in kebutuhan responses.
type KebutuhanMenuRef struct {
	ID       int64  `json:"id"`
	Tanggal  string `json:"tanggal"`
	NamaMenu string `json:"nama_menu"`
	Status   string `json:"status"`
}

// KebutuhanItemResponse is one computed row.
type KebutuhanItemResponse struct {
	BahanID      int64   `json:"bahan_id"`
	Nama         string  `json:"nama"`
	Kategori     string  `json:"kategori"`
	Satuan       string  `json:"satuan"`
	GramPerPorsi float64 `json:"gram_per_porsi"`
	TotalGram    float64 `json:"total_gram"`
	Qty          float64 `json:"qty"`
	MudahRusak   bool    `json:"mudah_rusak"`
}

// KebutuhanResponse is the MVP-002.6 success payload.
type KebutuhanResponse struct {
	Menu           KebutuhanMenuRef        `json:"menu"`
	TargetPorsi    int                     `json:"target_porsi"`
	CadanganPersen float64                 `json:"cadangan_persen"`
	Items          []KebutuhanItemResponse `json:"items"`
}

func toApprovalResponse(res *application.ApprovalResult) ApprovalResponse {
	items := make([]MenuBahanResponse, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, MenuBahanResponse{BahanID: it.BahanID, Nama: it.BahanNama, GramPerPorsi: it.GramPerPorsi})
	}
	return ApprovalResponse{
		ID: res.Menu.ID, Tanggal: res.Menu.Tanggal.Format("2006-01-02"), NamaMenu: res.Menu.NamaMenu,
		Gizi:        MenuGiziResponse{EnergiKkal: res.Menu.EnergiKkal, ProteinG: res.Menu.ProteinG, KarbohidratG: res.Menu.KarbohidratG, LemakG: res.Menu.LemakG},
		TargetPorsi: res.Menu.TargetPorsi, Status: res.Menu.Status, Bahan: items,
		DibuatOleh: res.Menu.DibuatOleh, DisetujuiOleh: res.Menu.DisetujuiOleh, DisetujuiAt: res.Menu.DisetujuiAt,
		CreatedAt: res.Menu.CreatedAt,
	}
}

func toKebutuhanResponse(res *application.KebutuhanResult) KebutuhanResponse {
	items := make([]KebutuhanItemResponse, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, KebutuhanItemResponse{
			BahanID: it.BahanID, Nama: it.Nama, Kategori: it.Kategori, Satuan: it.Satuan,
			GramPerPorsi: it.GramPerPorsi, TotalGram: it.TotalGram, Qty: it.Qty, MudahRusak: it.MudahRusak,
		})
	}
	return KebutuhanResponse{
		Menu: KebutuhanMenuRef{
			ID: res.Menu.ID, Tanggal: res.Menu.Tanggal.Format("2006-01-02"),
			NamaMenu: res.Menu.NamaMenu, Status: res.Menu.Status,
		},
		TargetPorsi: res.TargetPorsi, CadanganPersen: res.CadanganPersen, Items: items,
	}
}

// CopyMenuRequest is POST /menus/{id}/copy input.
type CopyMenuRequest struct {
	TanggalTujuan []string `json:"tanggal_tujuan"`
}

// CopyMenuResponse is the MVP-002.7 success payload.
type CopyMenuResponse struct {
	Data     []CreateMenuResponse `json:"data"`
	Warnings []string             `json:"warnings"`
}

func toCopyMenuResponse(res *application.CopyMenuResult) CopyMenuResponse {
	data := make([]CreateMenuResponse, 0, len(res.Menus))
	for i, m := range res.Menus {
		items := make([]MenuBahanResponse, 0, len(res.Items[i]))
		for _, it := range res.Items[i] {
			items = append(items, MenuBahanResponse{BahanID: it.BahanID, Nama: it.BahanNama, GramPerPorsi: it.GramPerPorsi})
		}
		data = append(data, CreateMenuResponse{
			ID: m.ID, Tanggal: m.Tanggal.Format("2006-01-02"), NamaMenu: m.NamaMenu,
			Gizi:        MenuGiziResponse{EnergiKkal: m.EnergiKkal, ProteinG: m.ProteinG, KarbohidratG: m.KarbohidratG, LemakG: m.LemakG},
			TargetPorsi: m.TargetPorsi, Status: m.Status, Bahan: items,
			DibuatOleh: m.DibuatOleh, CreatedAt: m.CreatedAt, Warnings: []string{},
		})
	}
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return CopyMenuResponse{Data: data, Warnings: warnings}
}
