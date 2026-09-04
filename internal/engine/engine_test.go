package engine

import (
	"errors"
	"testing"
)

func TestValidateUserOnly(t *testing.T) {
	if err := (Connection{User: "root"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Connection{}).Validate(); !errors.Is(err, ErrInvalidConnection) {
		t.Fatalf("got %v", err)
	}
}

func TestRequireDatabase(t *testing.T) {
	if err := (Connection{User: "root"}).RequireDatabase(); !errors.Is(err, ErrInvalidConnection) {
		t.Fatalf("got %v", err)
	}
	if err := (Connection{User: "root", Database: "app"}).RequireDatabase(); err != nil {
		t.Fatal(err)
	}
}

func TestLineNames(t *testing.T) {
	got := LineNames("mysql\n\napp\n")
	if len(got) != 2 || got[0] != "mysql" || got[1] != "app" {
		t.Fatalf("%q", got)
	}
}
