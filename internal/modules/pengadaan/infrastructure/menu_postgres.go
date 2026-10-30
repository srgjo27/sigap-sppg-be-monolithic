package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// MenuPostgres implements MenuRepository
type MenuPostgres struct {
	Pool *pgxpool.Pool
}

func NewMenuPostgres(pool *pgxpool.Pool) *MenuPostgres {
	return &MenuPostgres{Pool: pool}
}

const menuColumns = `id, sppg_id, tanggal, nama_menu, energi_kkal, protein_g, karbohidrat_g, lemak_g, target_porsi, catatan, dibuat_oleh, status, disetujui_oleh, disetujui_at, created_at, updated_at`

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

func (p *MenuPostgres) ListByScope(ctx context.Context, sppgID *int64, dari, sampai time.Time, status *string) ([]domain.Menu, error) {
	const q = `SELECT ` + menuColumns + ` FROM menu WHERE ($1::bigint IS NULL OR sppg_id=$1) AND tanggal >= $2 AND tanggal <= $3 AND ($4::menu_status IS NULL OR status=$4::menu_status) ORDER BY tanggal ASC, id ASC`
	var statusVal any
	if status != nil {
		statusVal = *status
	}
	var sppgVal any
	if sppgID != nil {
		sppgVal = *sppgID
	}
	rows, err := p.Pool.Query(ctx, q, sppgVal, dari.Format("2006-01-02"), sampai.Format("2006-01-02"), statusVal)
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

func (p *MenuPostgres) ListItems(ctx context.Context, menuIDs []int64) (map[int64][]domain.MenuBahan, error) {
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

func (p *MenuPostgres) FindByID(ctx context.Context, id int64) (*domain.Menu, error) {
	const q = `SELECT ` + menuColumns + ` FROM menu WHERE id=$1 LIMIT 1`
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

// FindByIDs finds multiple menus by IDs.
func (p *MenuPostgres) FindByIDs(ctx context.Context, ids []int64) ([]*domain.Menu, error) {
	if len(ids) == 0 {
		return []*domain.Menu{}, nil
	}
	const q = `SELECT ` + menuColumns + ` FROM menu WHERE id = ANY($1)`
	rows, err := p.Pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("find menus by ids: %w", err)
	}
	defer rows.Close()
	var out []*domain.Menu
	for rows.Next() {
		m, err := scanMenu(rows)
		if err != nil {
			return nil, fmt.Errorf("scan menu: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows menus: %w", err)
	}
	return out, nil
}
