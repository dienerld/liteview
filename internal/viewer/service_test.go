package viewer_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"liteview/internal/recents"
	"liteview/internal/rows"
	"liteview/internal/viewer"
)

func makeDB(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "app.db")
	c, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, err = c.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE);
		CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id));
		INSERT INTO users (email) VALUES ('a@x.com');`)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newService(t *testing.T, cli string) (*viewer.Service, *recents.Store) {
	t.Helper()
	rc := recents.New(filepath.Join(t.TempDir(), "recents.json"))
	return viewer.New(viewer.Options{Recents: rc, CLIPath: cli}), rc
}

func TestNoDatabaseOpen(t *testing.T) {
	s, _ := newService(t, "")
	if _, err := s.ListTables(); !errors.Is(err, viewer.ErrNoDatabase) {
		t.Fatalf("expected ErrNoDatabase, got %v", err)
	}
	if s.CurrentDB() != nil {
		t.Fatal("CurrentDB must be nil")
	}
}

func TestOpenPath_recordsRecentAndServesData(t *testing.T) {
	s, rc := newService(t, "")
	p := makeDB(t)
	info, err := s.OpenPath(p)
	if err != nil || info.Name != "app.db" {
		t.Fatalf("OpenPath: %+v %v", info, err)
	}
	if got := rc.List(); len(got) != 1 || got[0] != p {
		t.Fatalf("recents wrong: %v", got)
	}
	tables, err := s.ListTables()
	if err != nil || len(tables) != 2 {
		t.Fatalf("ListTables: %+v %v", tables, err)
	}
	page, err := s.QueryRows(rows.Query{Table: "users"})
	if err != nil || page.Total != 1 {
		t.Fatalf("QueryRows: %+v %v", page, err)
	}
}

func TestOpenPath_failureKeepsPreviousDatabase(t *testing.T) {
	s, _ := newService(t, "")
	if _, err := s.OpenPath(makeDB(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenPath("/definitely/missing.db"); err == nil {
		t.Fatal("expected error")
	}
	if s.CurrentDB() == nil {
		t.Fatal("previous database must stay open after a failed open")
	}
}

func TestCRUDRoundTrip(t *testing.T) {
	s, _ := newService(t, "")
	s.OpenPath(makeDB(t))
	key, err := s.InsertRow("users", map[string]any{"email": "b@x.com"})
	if err != nil || key["id"] != int64(2) {
		t.Fatalf("InsertRow: %v %v", key, err)
	}
	if err := s.UpdateRow("users", key, map[string]any{"email": "bb@x.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertRow("posts", map[string]any{"user_id": float64(2)}); err != nil {
		t.Fatal(err)
	}
	refs, err := s.References("users", key)
	if err != nil || len(refs) != 1 || refs[0].Count != 1 {
		t.Fatalf("References: %+v %v", refs, err)
	}
	if err := s.DeleteRow("users", key); err == nil {
		t.Fatal("delete must be blocked by FK")
	}
}

func TestInitialDB(t *testing.T) {
	p := makeDB(t)

	s, _ := newService(t, p) // CLI path wins
	if info, err := s.InitialDB(); err != nil || info == nil {
		t.Fatalf("CLI open: %+v %v", info, err)
	}

	s, _ = newService(t, "/missing.db") // CLI error is reported
	if _, err := s.InitialDB(); err == nil {
		t.Fatal("bad CLI path must return an error")
	}

	s, rc := newService(t, "") // last recent is reopened silently
	rc.Add(p)
	if info, err := s.InitialDB(); err != nil || info == nil {
		t.Fatalf("recent reopen: %+v %v", info, err)
	}

	s, rc = newService(t, "")
	rc.Add("/gone.db") // missing recent: no error, no db
	if info, err := s.InitialDB(); err != nil || info != nil {
		t.Fatalf("missing recent: %+v %v", info, err)
	}
}

func TestOpenDialog(t *testing.T) {
	rc := recents.New(filepath.Join(t.TempDir(), "recents.json"))

	s := viewer.New(viewer.Options{Recents: rc, Pick: func() (string, error) { return "", nil }})
	if info, err := s.OpenDialog(); info != nil || err != nil {
		t.Fatalf("cancelled dialog must yield nil, nil; got %+v %v", info, err)
	}

	boom := errors.New("boom")
	s = viewer.New(viewer.Options{Recents: rc, Pick: func() (string, error) { return "", boom }})
	if _, err := s.OpenDialog(); !errors.Is(err, boom) {
		t.Fatalf("picker error must propagate, got %v", err)
	}

	p := makeDB(t)
	s = viewer.New(viewer.Options{Recents: rc, Pick: func() (string, error) { return p, nil }})
	if info, err := s.OpenDialog(); err != nil || info == nil {
		t.Fatalf("picked path must open: %+v %v", info, err)
	}

	s = viewer.New(viewer.Options{Recents: rc})
	if _, err := s.OpenDialog(); err == nil {
		t.Fatal("missing picker must error")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEmptyResultsMarshalAsArrays(t *testing.T) {
	s, _ := newService(t, "")
	if got := mustJSON(t, s.Recents()); got != "[]" {
		t.Fatalf("Recents JSON = %s", got)
	}

	p := filepath.Join(t.TempDir(), "empty.db")
	c, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(`DROP TABLE t`); err != nil {
		t.Fatal(err)
	}
	c.Close()
	if _, err := s.OpenPath(p); err != nil {
		t.Fatal(err)
	}
	tables, err := s.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if got := mustJSON(t, tables); got != "[]" {
		t.Fatalf("ListTables JSON = %s", got)
	}
}

func TestReferencesEmptyMarshalsAsArray(t *testing.T) {
	s, _ := newService(t, "")
	if _, err := s.OpenPath(makeDB(t)); err != nil {
		t.Fatal(err)
	}
	refs, err := s.References("users", map[string]any{"id": int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if got := mustJSON(t, refs); got == "null" {
		t.Fatalf("References JSON = %s", got)
	}
}

func TestMarshalError(t *testing.T) {
	ce := &rows.ConstraintError{Kind: "NOT NULL", Columns: []string{"email"}, Msg: "email é obrigatório"}
	b := viewer.MarshalError(ce)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("not JSON: %s", b)
	}
	if got["type"] != "constraint" || got["kind"] != "NOT NULL" || got["message"] != "email é obrigatório" {
		t.Fatalf("payload wrong: %v", got)
	}
	if viewer.MarshalError(errors.New("boom")) != nil {
		t.Fatal("non-constraint errors must fall back to default handling")
	}
}

func TestMarshalError_emptyColumnsIsArray(t *testing.T) {
	for _, cols := range [][]string{{}, nil} {
		b := viewer.MarshalError(&rows.ConstraintError{Kind: "FOREIGN KEY", Columns: cols, Msg: "x"})
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatalf("not JSON: %s", b)
		}
		if string(raw["columns"]) != "[]" {
			t.Fatalf("columns = %s (input %#v)", raw["columns"], cols)
		}
	}
}
