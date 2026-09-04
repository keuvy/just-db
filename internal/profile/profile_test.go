package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"just-db/internal/engine"
)

func sample(name string) Record {
	return Record{
		Name:   name,
		Engine: engine.Postgres,
		Connection: engine.Connection{
			Host:     "db.example",
			Port:     5432,
			User:     "app",
			Password: "s3cret",
			Database: "shop",
			SSLMode:  "disable",
		},
	}
}

func TestPutGetListDelete(t *testing.T) {
	t.Setenv(envKey, "")
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := sample("prod")
	if err := st.Put(rec); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.Connection.Password != "s3cret" || got.Engine != engine.Postgres || got.Connection.Host != "db.example" {
		t.Fatalf("got %+v", got)
	}
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "prod" || list[0].User != "app" {
		t.Fatalf("list %+v", list)
	}
	raw, err := json.Marshal(list[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("s3cret")) {
		t.Fatalf("list leaked password: %s", raw)
	}
	if err := st.Delete("prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get("prod"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestReplaceKeepsName(t *testing.T) {
	t.Setenv(envKey, "")
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := sample("shop")
	if err := st.Put(rec); err != nil {
		t.Fatal(err)
	}
	rec.Connection.Host = "other.example"
	rec.Connection.Password = "newpass"
	if err := st.Put(rec); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get("shop")
	if err != nil {
		t.Fatal(err)
	}
	if got.Connection.Host != "other.example" || got.Connection.Password != "newpass" {
		t.Fatalf("got %+v", got)
	}
}

func TestFilesAreNotPlainJSON(t *testing.T) {
	t.Setenv(envKey, "")
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Put(sample("prod")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(Dir(dir), "prod.jdb"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("s3cret")) || bytes.Contains(raw, []byte("db.example")) {
		t.Fatalf("plaintext leaked into %q", raw)
	}
	if !bytes.HasPrefix(raw, []byte(magic)) {
		t.Fatalf("missing magic prefix")
	}
}

func TestEnvKeySharedAndWrongKeyFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envKey, "unit-test-passphrase")
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Put(sample("prod")); err != nil {
		t.Fatal(err)
	}
	st2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st2.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.Connection.Password != "s3cret" {
		t.Fatalf("got %+v", got)
	}
	t.Setenv(envKey, "other-passphrase")
	st3, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st3.Get("prod"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("wrong key: %v", err)
	}
}

func TestHexEnvKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envKey, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Put(sample("hex")); err != nil {
		t.Fatal(err)
	}
	st2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st2.Get("hex"); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidName(t *testing.T) {
	t.Setenv(envKey, "")
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "has space", "../etc", "key", "bad/name", string(make([]byte, 70))} {
		rec := sample("x")
		rec.Name = name
		if err := st.Put(rec); !errors.Is(err, ErrInvalidName) {
			t.Errorf("name %q: %v", name, err)
		}
	}
}

func TestPutRequiresConnection(t *testing.T) {
	t.Setenv(envKey, "")
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := sample("prod")
	rec.Connection.User = ""
	if err := st.Put(rec); !errors.Is(err, engine.ErrInvalidConnection) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteMissing(t *testing.T) {
	t.Setenv(envKey, "")
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestListSorted(t *testing.T) {
	t.Setenv(envKey, "")
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zeta", "alpha", "mid"} {
		if err := st.Put(sample(name)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Name != "alpha" || list[1].Name != "mid" || list[2].Name != "zeta" {
		t.Fatalf("%+v", list)
	}
}
