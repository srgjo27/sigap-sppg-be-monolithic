package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	authdomain "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/domain"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/domain"
)

// Postgres implements menu ports using pgx.
// Schema source of truth: db/schema.sql (bahan, menu, menu_bahan, sekolah, sppg, audit_log).
type Postgres struct {
	Pool *pgxpool.Pool
}

// NewPostgres builds stores around an existing pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{Pool: pool}
}

const bahanColumns = `id, nama, kategori, satuan, gram_per_satuan, mudah_rusak, suhu_simpan_maks, aktif, created_at, updated_at`

func scanBahan(row interface{ Scan(...any) error }) (*domain.Bahan, error) {
	var b domain.Bahan
	var suhu sql.NullFloat64
	var created, updated sql.NullTime
	var id sql.NullInt64
	var nama, kategori, satuan sql.NullString
	var gram sql.NullFloat64
	var rusak, aktif sql.NullBool
	if err := row.Scan(&id, &nama, &kategori, &satuan, &gram, &rusak, &suhu, &aktif, &created, &updated); err != nil {
		return nil, err
	}
	b.ID = id.Int64
	b.Nama = nama.String
	b.Kategori = kategori.String
	b.Satuan = satuan.String
	b.GramPerSatuan = gram.Float64
	b.MudahRusak = rusak.Bool
	if suhu.Valid {
		v := suhu.Float64
		b.SuhuSimpanMaks = &v
	}
	b.Aktif = aktif.Bool
	if created.Valid {
		b.CreatedAt = created.Time
	}
	if updated.Valid {
		b.UpdatedAt = updated.Time
	}
	return &b, nil
}

// Create inserts a bahan.
func (p *Postgres) Create(ctx context.Context, b *domain.Bahan) (*domain.Bahan, error) {
	const q = `INSERT INTO bahan (nama, kategori, satuan, gram_per_satuan, mudah_rusak, suhu_simpan_maks, aktif, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,true,now(),now()) RETURNING ` + bahanColumns
	row := p.Pool.QueryRow(ctx, q, b.Nama, b.Kategori, b.Satuan, b.GramPerSatuan, b.MudahRusak, nullableFloat(b.SuhuSimpanMaks))
	created, err := scanBahan(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrBahanExists
		}
		return nil, fmt.Errorf("insert bahan: %w", err)
	}
	return created, nil
}

// FindByID looks up a bahan.
func (p *Postgres) FindByID(ctx context.Context, id int64) (*domain.Bahan, error) {
	const q = `SELECT ` + bahanColumns + ` FROM bahan WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	b, err := scanBahan(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find bahan: %w", err)
	}
	return b, nil
}

// FindByNameInsensitive looks up by case-insensitive nama.
func (p *Postgres) FindByNameInsensitive(ctx context.Context, nama string) (*domain.Bahan, error) {
	const q = `SELECT ` + bahanColumns + ` FROM bahan WHERE lower(nama)=lower($1) LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, nama)
	b, err := scanBahan(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find bahan by name: %w", err)
	}
	return b, nil
}

// Update persists a bahan.
func (p *Postgres) Update(ctx context.Context, b *domain.Bahan) (*domain.Bahan, error) {
	const q = `UPDATE bahan SET nama=$2, kategori=$3, satuan=$4, gram_per_satuan=$5, mudah_rusak=$6,
		suhu_simpan_maks=$7, aktif=$8, updated_at=now() WHERE id=$1 RETURNING ` + bahanColumns
	row := p.Pool.QueryRow(ctx, q, b.ID, b.Nama, b.Kategori, b.Satuan, b.GramPerSatuan, b.MudahRusak, nullableFloat(b.SuhuSimpanMaks), b.Aktif)
	updated, err := scanBahan(row)
	if err != nil {
		if isNoRows(err) {
			return nil, domain.ErrBahanNotFound
		}
		if isUniqueViolation(err) {
			return nil, domain.ErrBahanExists
		}
		return nil, fmt.Errorf("update bahan: %w", err)
	}
	return updated, nil
}

// List returns filtered bahan rows.
func (p *Postgres) List(ctx context.Context, filter domain.BahanFilter) ([]domain.Bahan, int, error) {
	filter.Normalize()
	where := `WHERE ($1::text IS NULL OR lower(nama) LIKE '%'||lower($1)||'%') AND ($2::text IS NULL OR kategori=$2) AND ($3::boolean IS NULL OR aktif=$3)`
	var qVal, katVal any
	var aktifVal any
	if filter.Q != nil && *filter.Q != "" {
		qVal = *filter.Q
	}
	if filter.Kategori != nil {
		katVal = *filter.Kategori
	}
	if filter.Aktif != nil {
		aktifVal = *filter.Aktif
	}
	countQ := `SELECT COUNT(*) FROM bahan ` + where
	var total int
	if err := p.Pool.QueryRow(ctx, countQ, qVal, katVal, aktifVal).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count bahan: %w", err)
	}
	listQ := `SELECT ` + bahanColumns + ` FROM bahan ` + where + ` ORDER BY id ASC LIMIT $4 OFFSET $5`
	rows, err := p.Pool.Query(ctx, listQ, qVal, katVal, aktifVal, filter.Limit, filter.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list bahan: %w", err)
	}
	defer rows.Close()
	var out []domain.Bahan
	for rows.Next() {
		var b domain.Bahan
		var suhu sql.NullFloat64
		var created, updated sql.NullTime
		var gram sql.NullFloat64
		var rusak, aktif sql.NullBool
		if err := rows.Scan(&b.ID, &b.Nama, &b.Kategori, &b.Satuan, &gram, &rusak, &suhu, &aktif, &created, &updated); err != nil {
			return nil, 0, fmt.Errorf("scan bahan: %w", err)
		}
		b.GramPerSatuan = gram.Float64
		b.MudahRusak = rusak.Bool
		if suhu.Valid {
			v := suhu.Float64
			b.SuhuSimpanMaks = &v
		}
		b.Aktif = aktif.Bool
		if created.Valid {
			b.CreatedAt = created.Time
		}
		if updated.Valid {
			b.UpdatedAt = updated.Time
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows bahan: %w", err)
	}
	if out == nil {
		out = []domain.Bahan{}
	}
	return out, total, nil
}

// CreateMenu stores menu and items in one transaction.
func (p *Postgres) CreateMenu(ctx context.Context, menu *domain.Menu, items []domain.MenuBahan) (*domain.Menu, []domain.MenuBahan, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const mq = `INSERT INTO menu (sppg_id, tanggal, nama_menu, energi_kkal, protein_g, karbohidrat_g, lemak_g, target_porsi, dibuat_oleh, status, catatan, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'draf',$10,now(),now())
		RETURNING id, sppg_id, tanggal, nama_menu, energi_kkal, protein_g, karbohidrat_g, lemak_g, target_porsi, catatan, dibuat_oleh, status, disetujui_oleh, disetujui_at, created_at, updated_at`
	row := tx.QueryRow(ctx, mq, menu.SPPGID, menu.Tanggal.Format("2006-01-02"), menu.NamaMenu,
		menu.EnergiKkal, menu.ProteinG, menu.KarbohidratG, menu.LemakG, menu.TargetPorsi, menu.DibuatOleh, nullableStr(menu.Catatan))
	created, err := scanMenu(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, domain.ErrMenuExists
		}
		return nil, nil, fmt.Errorf("insert menu: %w", err)
	}
	stored := make([]domain.MenuBahan, 0, len(items))
	for _, it := range items {
		var id int64
		var nama string
		// menu_bahan has UNIQUE(menu_id, bahan_id); inactive check already done in service.
		const iq = `INSERT INTO menu_bahan (menu_id, bahan_id, gram_per_porsi) VALUES ($1,$2,$3) RETURNING id`
		if err := tx.QueryRow(ctx, iq, created.ID, it.BahanID, it.GramPerPorsi).Scan(&id); err != nil {
			if isUniqueViolation(err) {
				return nil, nil, domain.ErrDuplicateBahan
			}
			return nil, nil, fmt.Errorf("insert menu_bahan: %w", err)
		}
		const nq = `SELECT nama FROM bahan WHERE id=$1`
		if err := tx.QueryRow(ctx, nq, it.BahanID).Scan(&nama); err != nil {
			return nil, nil, fmt.Errorf("resolve bahan nama: %w", err)
		}
		stored = append(stored, domain.MenuBahan{ID: id, MenuID: created.ID, BahanID: it.BahanID, BahanNama: nama, GramPerPorsi: it.GramPerPorsi})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit menu: %w", err)
	}
	return created, stored, nil
}

func scanMenu(row interface{ Scan(...any) error }) (*domain.Menu, error) {
	var m domain.Menu
	var tanggal time.Time
	var catatan sql.NullString
	var disetujuiOleh sql.NullInt64
	var disetujuiAt sql.NullTime
	var created, updated sql.NullTime
	var energi, protein, karbo, lemak sql.NullFloat64
	var target sql.NullInt32
	var status sql.NullString
	if err := row.Scan(&m.ID, &m.SPPGID, &tanggal, &m.NamaMenu, &energi, &protein, &karbo, &lemak, &target, &catatan, &m.DibuatOleh, &status, &disetujuiOleh, &disetujuiAt, &created, &updated); err != nil {
		return nil, err
	}
	m.Tanggal = tanggal
	m.EnergiKkal = energi.Float64
	m.ProteinG = protein.Float64
	m.KarbohidratG = karbo.Float64
	m.LemakG = lemak.Float64
	m.TargetPorsi = int(target.Int32)
	if catatan.Valid {
		v := catatan.String
		m.Catatan = &v
	}
	m.Status = status.String
	if disetujuiOleh.Valid {
		v := disetujuiOleh.Int64
		m.DisetujuiOleh = &v
	}
	if disetujuiAt.Valid {
		v := disetujuiAt.Time
		m.DisetujuiAt = &v
	}
	if created.Valid {
		m.CreatedAt = created.Time
	}
	if updated.Valid {
		m.UpdatedAt = updated.Time
	}
	return &m, nil
}

// ExistsBySPPGDate checks UNIQUE(sppg_id, tanggal).
func (p *Postgres) ExistsBySPPGDate(ctx context.Context, sppgID int64, tanggal time.Time) (bool, error) {
	var exists bool
	if err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM menu WHERE sppg_id=$1 AND tanggal=$2)`, sppgID, tanggal.Format("2006-01-02")).Scan(&exists); err != nil {
		return false, fmt.Errorf("check menu date: %w", err)
	}
	return exists, nil
}

// GetCapacity resolves sppg capacity.
func (p *Postgres) GetCapacity(ctx context.Context, sppgID int64) (*int, bool, error) {
	var cap sql.NullInt32
	if err := p.Pool.QueryRow(ctx, `SELECT kapasitas_porsi FROM sppg WHERE id=$1`, sppgID).Scan(&cap); err != nil {
		if isNoRows(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get sppg capacity: %w", err)
	}
	if !cap.Valid {
		return nil, true, nil
	}
	v := int(cap.Int32)
	return &v, true, nil
}

// SumRecipients sums sekolah.jumlah_penerima for an SPPG.
func (p *Postgres) SumRecipients(ctx context.Context, sppgID int64) (int, error) {
	var sum sql.NullInt64
	if err := p.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(jumlah_penerima),0) FROM sekolah WHERE sppg_id=$1`, sppgID).Scan(&sum); err != nil {
		return 0, fmt.Errorf("sum recipients: %w", err)
	}
	return int(sum.Int64), nil
}

// FindOwner returns sekolah ownership for pic_sekolah scoping.
func (p *Postgres) FindOwner(ctx context.Context, sekolahID int64) (bool, int64, error) {
	var sppgID int64
	if err := p.Pool.QueryRow(ctx, `SELECT sppg_id FROM sekolah WHERE id=$1`, sekolahID).Scan(&sppgID); err != nil {
		if isNoRows(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("find sekolah owner: %w", err)
	}
	return true, sppgID, nil
}

// FindMenuByID looks up a menu.
func (p *Postgres) FindMenuByID(ctx context.Context, id int64) (*domain.Menu, error) {
	const q = `SELECT id, sppg_id, tanggal, nama_menu, energi_kkal, protein_g, karbohidrat_g, lemak_g, target_porsi, catatan, dibuat_oleh, status, disetujui_oleh, disetujui_at, created_at, updated_at FROM menu WHERE id=$1 LIMIT 1`
	row := p.Pool.QueryRow(ctx, q, id)
	m, err := scanMenu(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find menu: %w", err)
	}
	return m, nil
}

// ListByScope returns menus ordered by tanggal ASC.
func (p *Postgres) ListByScope(ctx context.Context, sppgID *int64, dari, sampai time.Time, status *string) ([]domain.Menu, error) {
	const q = `SELECT id, sppg_id, tanggal, nama_menu, energi_kkal, protein_g, karbohidrat_g, lemak_g, target_porsi, catatan, dibuat_oleh, status, disetujui_oleh, disetujui_at, created_at, updated_at
		FROM menu WHERE ($1::bigint IS NULL OR sppg_id=$1) AND tanggal >= $2 AND tanggal <= $3 AND ($4::text IS NULL OR status=$4)
		ORDER BY tanggal ASC, id ASC`
	var statusVal any
	if status != nil {
		statusVal = *status
	}
	rows, err := p.Pool.Query(ctx, q, nullableInt(sppgID), dari.Format("2006-01-02"), sampai.Format("2006-01-02"), statusVal)
	if err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}
	defer rows.Close()
	var out []domain.Menu
	for rows.Next() {
		m, err := scanMenu(rows)
		if err != nil {
			return nil, fmt.Errorf("scan menu: %w", err)
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows menus: %w", err)
	}
	if out == nil {
		out = []domain.Menu{}
	}
	return out, nil
}

// ListItems returns composition keyed by menu id.
func (p *Postgres) ListItems(ctx context.Context, menuIDs []int64) (map[int64][]domain.MenuBahan, error) {
	out := map[int64][]domain.MenuBahan{}
	for _, id := range menuIDs {
		out[id] = []domain.MenuBahan{}
	}
	if len(menuIDs) == 0 {
		return out, nil
	}
	const q = `SELECT mb.id, mb.menu_id, mb.bahan_id, b.nama, mb.gram_per_porsi FROM menu_bahan mb JOIN bahan b ON b.id=mb.bahan_id WHERE mb.menu_id = ANY($1) ORDER BY mb.id ASC`
	rows, err := p.Pool.Query(ctx, q, menuIDs)
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it domain.MenuBahan
		if err := rows.Scan(&it.ID, &it.MenuID, &it.BahanID, &it.BahanNama, &it.GramPerPorsi); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		out[it.MenuID] = append(out[it.MenuID], it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows menu items: %w", err)
	}
	return out, nil
}

// UpdateMenu persists menu fields and optionally replaces composition atomically.
func (p *Postgres) UpdateMenu(ctx context.Context, menu *domain.Menu, replaceItems *[]domain.MenuBahan) (*domain.Menu, []domain.MenuBahan, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const q = `UPDATE menu SET nama_menu=$2, energi_kkal=$3, protein_g=$4, karbohidrat_g=$5, lemak_g=$6, target_porsi=$7, catatan=$8, updated_at=now()
		WHERE id=$1 RETURNING id, sppg_id, tanggal, nama_menu, energi_kkal, protein_g, karbohidrat_g, lemak_g, target_porsi, catatan, dibuat_oleh, status, disetujui_oleh, disetujui_at, created_at, updated_at`
	row := tx.QueryRow(ctx, q, menu.ID, menu.NamaMenu, menu.EnergiKkal, menu.ProteinG, menu.KarbohidratG, menu.LemakG, menu.TargetPorsi, nullableStr(menu.Catatan))
	updated, err := scanMenu(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil, domain.ErrMenuNotFound
		}
		return nil, nil, fmt.Errorf("update menu: %w", err)
	}
	var stored []domain.MenuBahan
	if replaceItems != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM menu_bahan WHERE menu_id=$1`, menu.ID); err != nil {
			return nil, nil, fmt.Errorf("replace menu_bahan: %w", err)
		}
		stored = make([]domain.MenuBahan, 0, len(*replaceItems))
		for _, it := range *replaceItems {
			var id int64
			var nama string
			const iq = `INSERT INTO menu_bahan (menu_id, bahan_id, gram_per_porsi) VALUES ($1,$2,$3) RETURNING id`
			if err := tx.QueryRow(ctx, iq, menu.ID, it.BahanID, it.GramPerPorsi).Scan(&id); err != nil {
				if isUniqueViolation(err) {
					return nil, nil, domain.ErrDuplicateBahan
				}
				return nil, nil, fmt.Errorf("insert menu_bahan: %w", err)
			}
			if err := tx.QueryRow(ctx, `SELECT nama FROM bahan WHERE id=$1`, it.BahanID).Scan(&nama); err != nil {
				return nil, nil, fmt.Errorf("resolve bahan nama: %w", err)
			}
			stored = append(stored, domain.MenuBahan{ID: id, MenuID: menu.ID, BahanID: it.BahanID, BahanNama: nama, GramPerPorsi: it.GramPerPorsi})
		}
	} else {
		const iq = `SELECT mb.id, mb.menu_id, mb.bahan_id, b.nama, mb.gram_per_porsi FROM menu_bahan mb JOIN bahan b ON b.id=mb.bahan_id WHERE mb.menu_id=$1 ORDER BY mb.id ASC`
		rows, err := tx.Query(ctx, iq, menu.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("list menu items: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var it domain.MenuBahan
			if err := rows.Scan(&it.ID, &it.MenuID, &it.BahanID, &it.BahanNama, &it.GramPerPorsi); err != nil {
				return nil, nil, fmt.Errorf("scan menu item: %w", err)
			}
			stored = append(stored, it)
		}
		if err := rows.Err(); err != nil {
			return nil, nil, fmt.Errorf("rows menu items: %w", err)
		}
	}
	if stored == nil {
		stored = []domain.MenuBahan{}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit menu: %w", err)
	}
	return updated, stored, nil
}

// DeleteMenu removes a menu; menu_bahan cascades.
func (p *Postgres) DeleteMenu(ctx context.Context, id int64) error {
	ct, err := p.Pool.Exec(ctx, `DELETE FROM menu WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete menu: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrMenuNotFound
	}
	return nil
}

// IsMenuUsed reports whether batch_produksi references the menu.
func (p *Postgres) IsMenuUsed(ctx context.Context, menuID int64) (bool, error) {
	var exists bool
	if err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM batch_produksi WHERE menu_id=$1)`, menuID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check menu usage: %w", err)
	}
	return exists, nil
}

// Append writes an audit entry.
func (p *Postgres) Append(ctx context.Context, e *authdomain.AuditEntry) error {
	const q = `INSERT INTO audit_log (user_id, aksi, tabel, record_id, data_lama, data_baru, ip_address)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::inet)`
	if _, err := p.Pool.Exec(ctx, q,
		nullableInt(e.UserID), e.Aksi, e.Tabel, nullableInt(e.RecordID),
		nullableStr(e.DataLama), nullableStr(e.DataBaru), nullableStr(e.IPAddress),
	); err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return nil
}

func nullableInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "no rows in result set"
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "already exists")
}
