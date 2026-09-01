package registry

import (
	"context"
	"testing"

	"just-db/internal/engine"
)

func TestInfosListsPostgresAndMySQL(t *testing.T) {
	infos, err := New().Infos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d engines, want 2", len(infos))
	}
	if infos[0].Name != engine.Postgres || infos[1].Name != engine.MySQL {
		t.Fatalf("unexpected engines: %+v", infos)
	}
	if infos[0].DefaultPort != 5432 || infos[1].DefaultPort != 3306 {
		t.Fatalf("unexpected default ports: %+v", infos)
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := New().Get("sqlite")
	if err == nil {
		t.Fatal("expected error")
	}
}
