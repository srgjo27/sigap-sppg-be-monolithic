package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/domain"
)

// This transaction double enforces pgx's closed-transaction behavior. Unlike
// application repository fakes, it exercises the actual SQL transaction flow.
type correctionTx struct {
	pgx.Tx
	currentStatus  string
	fulfilled      bool
	manuallyClosed bool
	failRead       string
	readErr        error
	commitErr      error
	committed      bool
	poRead         bool
	detailRead     bool
	statusUpdated  bool
}

type correctionRow func(...any) error

func (r correctionRow) Scan(dest ...any) error { return r(dest...) }

func (tx *correctionTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	return correctionRow(func(dest ...any) error {
		if tx.committed {
			return pgx.ErrTxClosed
		}
		switch {
		case strings.Contains(query, "FOR UPDATE"):
			*dest[0].(*int64) = 67
			*dest[1].(*int64) = 34
			*dest[2].(*string) = tx.currentStatus
			*dest[3].(*sql.NullInt64) = sql.NullInt64{Int64: 3, Valid: tx.manuallyClosed}
		case strings.Contains(query, "UPDATE penerimaan_bahan"):
			*dest[0].(*int64) = 46
		case strings.Contains(query, "UPDATE purchase_order"), strings.Contains(query, "FROM purchase_order WHERE"):
			if tx.failRead == "po" {
				return tx.readErr
			}
			tx.poRead = true
			tx.statusUpdated = strings.Contains(query, "UPDATE purchase_order")
			status := tx.currentStatus
			if tx.statusUpdated {
				status = domain.POStatusDiterimaSebagian
				if tx.fulfilled {
					status = domain.POStatusSelesai
				}
			}
			*dest[0].(*int64) = 34
			*dest[8].(*sql.NullString) = sql.NullString{String: status, Valid: true}
		case strings.Contains(query, "FROM penerimaan_bahan pb"):
			if tx.failRead == "detail" {
				return tx.readErr
			}
			tx.detailRead = true
			*dest[0].(*int64) = 46
			*dest[9].(*float64) = 5
		default:
			return errors.New("unexpected query: " + query)
		}
		return nil
	})
}

type correctionRows struct {
	pgx.Rows
	row  correctionRow
	done bool
}

func (r *correctionRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}

func (r *correctionRows) Scan(dest ...any) error { return r.row(dest...) }
func (r *correctionRows) Close()                 {}
func (r *correctionRows) Err() error             { return nil }

func (tx *correctionTx) Query(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
	if tx.committed {
		return nil, pgx.ErrTxClosed
	}
	switch {
	case strings.Contains(query, "SELECT id FROM po_item"):
		return &correctionRows{row: func(dest ...any) error {
			*dest[0].(*int64) = 67
			return nil
		}}, nil
	case strings.Contains(query, "FROM v_po_item_rekap"):
		return &correctionRows{row: func(dest ...any) error {
			*dest[0].(*int64) = 67
			*dest[4].(*sql.NullBool) = sql.NullBool{Bool: tx.fulfilled, Valid: true}
			return nil
		}}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (tx *correctionTx) Commit(context.Context) error {
	if !tx.poRead || !tx.detailRead {
		return errors.New("commit before reading response")
	}
	if tx.commitErr != nil {
		return tx.commitErr
	}
	tx.committed = true
	return nil
}

func correctionInput() domain.KoreksiPenerimaanInput {
	qtyDiterima, qtyDitolak, kondisi := 5.0, 0.0, domain.KondisiBaik
	return domain.KoreksiPenerimaanInput{
		QtyDiterima: &qtyDiterima,
		QtyDitolak:  &qtyDitolak,
		Kondisi:     &kondisi,
	}
}

func TestKoreksiPenerimaanTransaction(t *testing.T) {
	for _, tc := range []struct {
		name, currentStatus, wantStatus       string
		fulfilled, manuallyClosed, wantUpdate bool
	}{
		{"unchanged partial PO", domain.POStatusDiterimaSebagian, domain.POStatusDiterimaSebagian, false, false, false},
		{"completed PO", domain.POStatusDiterimaSebagian, domain.POStatusSelesai, true, false, true},
		{"reopened PO", domain.POStatusSelesai, domain.POStatusDiterimaSebagian, false, false, true},
		{"manually closed PO", domain.POStatusSelesai, domain.POStatusSelesai, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &correctionTx{currentStatus: tc.currentStatus, fulfilled: tc.fulfilled, manuallyClosed: tc.manuallyClosed}
			detail, po, err := koreksiPenerimaanTx(context.Background(), tx, 46, correctionInput(), 3, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if !tx.committed || detail.ID != 46 || detail.QtyDiterima != 5 || po.ID != 34 || po.Status != tc.wantStatus {
				t.Fatalf("unexpected result: committed=%v detail=%+v po=%+v", tx.committed, detail, po)
			}
			if tx.statusUpdated != tc.wantUpdate {
				t.Fatalf("status updated=%v, want %v", tx.statusUpdated, tc.wantUpdate)
			}
		})
	}
}

func TestKoreksiPenerimaanReadFailureDoesNotCommit(t *testing.T) {
	for _, read := range []string{"po", "detail"} {
		t.Run(read, func(t *testing.T) {
			readErr := errors.New("read failed")
			tx := &correctionTx{currentStatus: domain.POStatusDiterimaSebagian, failRead: read, readErr: readErr}
			detail, po, err := koreksiPenerimaanTx(context.Background(), tx, 46, correctionInput(), 3, time.Now())
			if !errors.Is(err, readErr) || tx.committed || detail != nil || po != nil {
				t.Fatalf("expected read error without commit: detail=%v po=%v err=%v committed=%v", detail, po, err, tx.committed)
			}
		})
	}
}

func TestKoreksiPenerimaanCommitFailure(t *testing.T) {
	commitErr := errors.New("commit failed")
	tx := &correctionTx{currentStatus: domain.POStatusDiterimaSebagian, commitErr: commitErr}
	detail, po, err := koreksiPenerimaanTx(context.Background(), tx, 46, correctionInput(), 3, time.Now())
	if !errors.Is(err, commitErr) || tx.committed || detail != nil || po != nil {
		t.Fatalf("expected commit error: detail=%v po=%v err=%v", detail, po, err)
	}
}
