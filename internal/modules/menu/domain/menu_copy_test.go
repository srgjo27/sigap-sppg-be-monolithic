package domain

import (
	"testing"
	"time"
)

func copyDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestValidateCopyMenuInput(t *testing.T) {
	today := copyDate("2026-10-09")
	if err := ValidateCopyMenuInput(CopyMenuInput{}, today); err == nil {
		t.Fatal("empty dates must fail")
	} else if ve, ok := err.(*ValidationError); !ok || ve.Fields["tanggal_tujuan"] == "" {
		t.Fatalf("expected tanggal_tujuan field, got %v", err)
	}
	many := make([]time.Time, 0, 8)
	for i := 1; i <= 8; i++ {
		many = append(many, copyDate("2026-10-09").AddDate(0, 0, i))
	}
	if err := ValidateCopyMenuInput(CopyMenuInput{TanggalTujuan: many}, today); err == nil {
		t.Fatal(">7 dates must fail")
	}
	dupDay := copyDate("2026-10-10")
	if err := ValidateCopyMenuInput(CopyMenuInput{TanggalTujuan: []time.Time{dupDay, dupDay}}, today); err == nil {
		t.Fatal("duplicate dates must fail")
	}
	if err := ValidateCopyMenuInput(CopyMenuInput{TanggalTujuan: []time.Time{copyDate("2026-10-08")}}, today); err == nil {
		t.Fatal("past date must fail")
	}
	week := []time.Time{copyDate("2026-10-10"), copyDate("2026-10-16")}
	if err := ValidateCopyMenuInput(CopyMenuInput{TanggalTujuan: week}, today); err != nil {
		t.Fatalf("valid week must pass: %v", err)
	}
}

func TestMenuDateConflictError(t *testing.T) {
	err := &MenuDateConflictError{Tanggal: "2026-10-12"}
	if err.Error() == "" {
		t.Fatal("conflict error must carry the date")
	}
}
