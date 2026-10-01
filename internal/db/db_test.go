package db_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"liteview/internal/db"
)

func makeDB(t *testing.T, path string) {
	t.Helper()
	c, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_existingDatabase(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ok.db")
	makeDB(t, p)
	conn, info, err := db.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if info.Name != "ok.db" || info.Path != p || info.ReadOnly {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestOpen_missingFileIsNotCreated(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.db")
	if _, _, err := db.Open(p); err == nil {
		t.Fatal("expected error for missing file")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file must not be created, stat err = %v", err)
	}
}

func TestOpen_notADatabase(t *testing.T) {
	p := filepath.Join(t.TempDir(), "text.db")
	content := make([]byte, 0, 300)
	for len(content) < 300 {
		content = append(content, []byte("this is plain text, not sqlite. ")...)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Open(p); err == nil {
		t.Fatal("expected error for non-sqlite file")
	}
}

func TestOpen_directoryIsRejected(t *testing.T) {
	if _, _, err := db.Open(t.TempDir()); err == nil {
		t.Fatal("expected error for directory")
	}
}

func TestOpen_enablesForeignKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fk.db")
	makeDB(t, p)
	conn, _, err := db.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var on int
	if err := conn.QueryRow(`PRAGMA foreign_keys`).Scan(&on); err != nil || on != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", on, err)
	}
}

func TestOpen_readOnlyFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	p := filepath.Join(t.TempDir(), "ro.db")
	makeDB(t, p)
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	conn, info, err := db.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if !info.ReadOnly {
		t.Fatal("expected ReadOnly = true")
	}
	if _, err := conn.Exec(`INSERT INTO t (v) VALUES ('x')`); err == nil {
		t.Fatal("write must fail on read-only database")
	}
}
