package domain

import (
	"testing"
	"time"
)

func TestValidatePhone(t *testing.T) {
	valid := []string{"081234567890", "0812345678", "+6281234567890", "6281234567890", "0812-3456-7890"}
	for _, p := range valid {
		if !ValidatePhone(p) {
			t.Errorf("expected valid phone %q", p)
		}
	}
	invalid := []string{"", "123", "0712345678", "+627123456789", "08abc", "08123", "081234567890123456"}
	for _, p := range invalid {
		if ValidatePhone(p) {
			t.Errorf("expected invalid phone %q", p)
		}
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	cases := []struct {
		pw    string
		valid bool
	}{
		{"password1", true},
		{"Pass1234", true},
		{"short1", false},
		{"onlyletters", false},
		{"12345678", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := ValidatePasswordPolicy(tc.pw); got != tc.valid {
			t.Errorf("password %q: got %v want %v", tc.pw, got, tc.valid)
		}
	}
}

func TestValidateCreate(t *testing.T) {
	sppg := int64(1)
	sekolah := int64(10)
	badRole := CreateUserInput{Nama: "A", Email: "bad", Peran: "nope", SPPGID: &sppg}
	if err := ValidateCreate(badRole); err == nil {
		t.Fatal("expected validation error")
	}
	ok := CreateUserInput{Nama: "Budi Santoso", Email: "BUDI@Example.com", Peran: RolePetugasDapur, SPPGID: &sppg}
	if err := ValidateCreate(ok); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	picMissing := CreateUserInput{Nama: "PIC Sekolah", Email: "pic@x.com", Peran: RolePICsekolah, SPPGID: &sppg}
	if err := ValidateCreate(picMissing); err == nil {
		t.Fatal("expected sekolah required for pic_sekolah")
	}
	picOK := CreateUserInput{Nama: "PIC Sekolah", Email: "pic@x.com", Peran: RolePICsekolah, SPPGID: &sppg, SekolahID: &sekolah}
	if err := ValidateCreate(picOK); err != nil {
		t.Fatalf("unexpected pic validation error: %v", err)
	}
	nonPicWithSekolah := CreateUserInput{Nama: "Dapur", Email: "d@x.com", Peran: RolePetugasDapur, SPPGID: &sppg, SekolahID: &sekolah}
	if err := ValidateCreate(nonPicWithSekolah); err == nil {
		t.Fatal("expected sekolah rejection for non-pic")
	}
}

func TestLockoutFlow(t *testing.T) {
	now := time.Now()
	u := &User{}
	for i := 0; i < 4; i++ {
		u.RecordFailedLogin(now)
		if u.IsLocked(now) {
			t.Fatalf("should not lock before 5 attempts (i=%d)", i)
		}
	}
	u.RecordFailedLogin(now)
	if !u.IsLocked(now) {
		t.Fatal("should lock after 5 attempts")
	}
	if !u.IsLocked(now.Add(14 * time.Minute)) {
		t.Fatal("should still be locked at 14m")
	}
	if u.IsLocked(now.Add(16 * time.Minute)) {
		t.Fatal("should unlock after 15m")
	}
	u.RecordSuccessfulLogin(now)
	if u.GagalLogin != 0 || u.IsLocked(now) {
		t.Fatal("successful login should reset lockout")
	}
}

func TestGenerateRandomPasswordMeetsPolicy(t *testing.T) {
	for i := 0; i < 20; i++ {
		pw, err := GenerateRandomPassword()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidatePasswordPolicy(pw) {
			t.Fatalf("generated password fails policy: %q", pw)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  USER@Example.COM "); got != "user@example.com" {
		t.Fatalf("got %q", got)
	}
}
