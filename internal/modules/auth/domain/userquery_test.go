package domain

import "testing"

func TestUserFilterNormalize(t *testing.T) {
	f := UserFilter{}
	f.Normalize()
	if f.Sort != UserSortNama || f.Page != 1 || f.Limit != 20 {
		t.Fatalf("defaults wrong: %+v", f)
	}
}

func TestUserFilterValidate(t *testing.T) {
	one := "x"
	f := UserFilter{Q: &one, Limit: 20}
	if err := f.Validate(); err == nil {
		t.Fatal("q with 1 char must fail")
	} else if ve, ok := err.(*ValidationError); !ok || ve.Fields["q"] == "" {
		t.Fatalf("expected q field, got %v", err)
	}
	two := "ab"
	f = UserFilter{Q: &two, Limit: 20}
	if err := f.Validate(); err != nil {
		t.Fatalf("q with 2 chars must pass: %v", err)
	}
	f = UserFilter{Peran: []Role{"bogus"}, Limit: 20}
	if err := f.Validate(); err == nil {
		t.Fatal("unknown peran must fail")
	}
	f = UserFilter{Peran: []Role{RoleAkuntan, RolePetugasDapur}, Limit: 20}
	if err := f.Validate(); err != nil {
		t.Fatalf("known peran must pass: %v", err)
	}
	f = UserFilter{Sort: "email", Limit: 20}
	if err := f.Validate(); err == nil {
		t.Fatal("unknown sort must fail")
	}
	for _, s := range []string{"", UserSortNama, UserSortMinusCreated, UserSortMinusLogin} {
		f = UserFilter{Sort: s, Limit: 20}
		if err := f.Validate(); err != nil {
			t.Fatalf("sort %q must pass: %v", s, err)
		}
	}
	f = UserFilter{Limit: 101}
	if err := f.Validate(); err == nil {
		t.Fatal("per_page > 100 must fail")
	} else if ve, ok := err.(*ValidationError); !ok || ve.Fields["per_page"] == "" {
		t.Fatalf("expected per_page field, got %v", err)
	}
}

func TestUserFilterOffset(t *testing.T) {
	f := UserFilter{Page: 3, Limit: 20}
	if f.Offset() != 40 {
		t.Fatalf("offset = %d", f.Offset())
	}
}
