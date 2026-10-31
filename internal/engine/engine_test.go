package engine

import (
	"errors"
	"strings"
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

func TestExportSelectionContent(t *testing.T) {
	cases := map[string][2]bool{"": {true, true}, "all": {true, true}, "schema": {true, false}, "DATA": {false, true}}
	for content, want := range cases {
		sel, err := ExportOptions{Content: content}.Selection()
		if err != nil || !sel.All || sel.Schema != want[0] || sel.Data != want[1] {
			t.Errorf("%q: %+v %v", content, sel, err)
		}
	}
	if _, err := (ExportOptions{Content: "neither"}).Selection(); !errors.Is(err, ErrInvalidExport) {
		t.Fatalf("expected ErrInvalidExport, got %v", err)
	}
	sel, err := ExportOptions{Tables: []string{" items ", ""}, Content: "schema"}.Selection()
	if err != nil || sel.All || len(sel.Tables) != 1 || sel.Tables[0] != (TableContent{Name: "items", Schema: true}) {
		t.Fatalf("tables: %+v %v", sel, err)
	}
	if sel, _ := (ExportOptions{Tables: []string{""}}).Selection(); !sel.All {
		t.Fatal("blank table names should mean every table")
	}
}

func TestExportSelectionPerTable(t *testing.T) {
	sel, err := ExportOptions{
		Tables:  []string{"ignored"},
		Content: "schema",
		PerTable: []TableContent{
			{Name: "both", Schema: true, Data: true},
			{Name: "skipped"},
			{Name: "rows", Data: true},
			{Name: "shape", Schema: true},
		},
	}.Selection()
	if err != nil || sel.All || !sel.Schema || !sel.Data {
		t.Fatalf("%+v %v", sel, err)
	}
	both, schemaOnly, dataOnly := sel.Groups()
	if strings.Join(both, ",") != "both" || strings.Join(schemaOnly, ",") != "shape" || strings.Join(dataOnly, ",") != "rows" {
		t.Fatalf("groups: %q %q %q", both, schemaOnly, dataOnly)
	}
	if strings.Join(sel.Names(), ",") != "both,rows,shape" {
		t.Fatalf("names: %q", sel.Names())
	}
	for _, bad := range [][]TableContent{
		{{Name: "a"}},
		{{Name: " ", Data: true}},
		{{Name: "a", Data: true}, {Name: "a", Schema: true}},
	} {
		if _, err := (ExportOptions{PerTable: bad}).Selection(); !errors.Is(err, ErrInvalidExport) {
			t.Errorf("%+v: expected ErrInvalidExport, got %v", bad, err)
		}
	}
}
