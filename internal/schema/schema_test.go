package schema_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"sqliteviewer/internal/schema"
	"sqliteviewer/internal/testdb"
)

const fixture = `
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE, manager_id INTEGER REFERENCES users(id));
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT DEFAULT 'untitled');
CREATE TABLE tags (a INTEGER, b INTEGER, label TEXT, PRIMARY KEY (a, b)) WITHOUT ROWID;
CREATE TABLE tag_notes (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER, FOREIGN KEY (a, b) REFERENCES tags(a, b));
CREATE TABLE log (msg TEXT);
CREATE TABLE implicit_child (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users);
CREATE TABLE "we""ird name" ("select" TEXT, "my col" INTEGER, PRIMARY KEY ("select"));
CREATE TABLE gen (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) VIRTUAL);
CREATE TABLE auto (id INTEGER PRIMARY KEY AUTOINCREMENT);
CREATE TABLE fk_broken (id INTEGER PRIMARY KEY, broken_ref INTEGER REFERENCES nonexistent);
CREATE TABLE reordered_order (y INTEGER, x INTEGER, FOREIGN KEY (y, x) REFERENCES tags(b, a));
CREATE VIEW v_posts AS SELECT id, title FROM posts;
`

func TestListTables(t *testing.T) {
	db := testdb.New(t, fixture)
	got, err := schema.ListTables(db)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, ti := range got {
		kinds[ti.Name] = ti.Kind
	}
	if kinds["users"] != "table" || kinds["v_posts"] != "view" || kinds[`we"ird name`] != "table" {
		t.Fatalf("unexpected kinds: %v", kinds)
	}
	if _, ok := kinds["sqlite_sequence"]; ok {
		t.Fatal("internal sqlite_ tables must be hidden")
	}
	if got[len(got)-1].Name != "v_posts" {
		t.Fatalf("views must come after tables, got last = %s", got[len(got)-1].Name)
	}
}

func TestListTables_emptyDatabaseReturnsNonNilSlice(t *testing.T) {
	db := testdb.New(t, "")
	got, err := schema.ListTables(db)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("empty ListTables must return non-nil empty slice, not nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(got))
	}
}

func TestGetTable_columnsAndKeys(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "posts")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.PrimaryKey, []string{"id"}) || !reflect.DeepEqual(s.KeyColumns, []string{"id"}) || s.UsesRowID || s.ReadOnly {
		t.Fatalf("keys wrong: %+v", s)
	}
	var title schema.Column
	for _, c := range s.Columns {
		if c.Name == "title" {
			title = c
		}
	}
	if title.Default == nil || *title.Default != "'untitled'" {
		t.Fatalf("default wrong: %+v", title)
	}
}

func TestGetTable_noPKUsesRowID(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "log")
	if err != nil {
		t.Fatal(err)
	}
	if !s.UsesRowID || !reflect.DeepEqual(s.KeyColumns, []string{"rowid"}) || s.ReadOnly {
		t.Fatalf("rowid handling wrong: %+v", s)
	}
}

func TestGetTable_withoutRowIDCompositePK(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "tags")
	if err != nil {
		t.Fatal(err)
	}
	if s.UsesRowID || !reflect.DeepEqual(s.KeyColumns, []string{"a", "b"}) {
		t.Fatalf("composite key wrong: %+v", s)
	}
}

func TestGetTable_viewIsReadOnly(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "v_posts")
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != "view" || !s.ReadOnly {
		t.Fatalf("view must be read-only: %+v", s)
	}
}

func TestGetTable_weirdIdentifiers(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, `we"ird name`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.PrimaryKey, []string{"select"}) || len(s.Columns) != 2 || s.Columns[1].Name != "my col" {
		t.Fatalf("weird identifiers wrong: %+v", s)
	}
}

func TestGetTable_generatedColumn(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "gen")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Columns) != 2 || s.Columns[0].Generated || !s.Columns[1].Generated {
		t.Fatalf("generated flag wrong: %+v", s.Columns)
	}
}

func TestGetTable_foreignKeysOutAndIn(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "users")
	if err != nil {
		t.Fatal(err)
	}
	// self reference goes out and comes in
	if len(s.ForeignKeys) != 1 {
		t.Fatalf("expected 1 outgoing FK, got %d: %+v", len(s.ForeignKeys), s.ForeignKeys)
	}
	if s.ForeignKeys[0].Table != "users" {
		t.Fatalf("outgoing wrong: %+v", s.ForeignKeys)
	}
	in := map[string]schema.IncomingFK{}
	for _, f := range s.Incoming {
		in[f.Table] = f
	}
	if !reflect.DeepEqual(in["posts"].From, []string{"user_id"}) || !reflect.DeepEqual(in["posts"].To, []string{"id"}) {
		t.Fatalf("incoming from posts wrong: %+v", in["posts"])
	}
	if _, ok := in["users"]; !ok {
		t.Fatal("self reference must appear as incoming")
	}
}

func TestGetTable_implicitFKTargetResolvesToPK(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "implicit_child")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(s.ForeignKeys))
	}
	fk := s.ForeignKeys[0]
	if fk.Table != "users" || !reflect.DeepEqual(fk.To, []string{"id"}) {
		t.Fatalf("implicit target not resolved: %+v", fk)
	}
	users, err := schema.GetTable(db, "users")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range users.Incoming {
		if f.Table == "implicit_child" && reflect.DeepEqual(f.To, []string{"id"}) {
			found = true
		}
	}
	if !found {
		t.Fatal("implicit FK must show up as incoming on the parent")
	}
}

func TestGetTable_compositeFK(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "tag_notes")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(s.ForeignKeys))
	}
	fk := s.ForeignKeys[0]
	if !reflect.DeepEqual(fk.From, []string{"a", "b"}) || !reflect.DeepEqual(fk.To, []string{"a", "b"}) {
		t.Fatalf("composite FK wrong: %+v", fk)
	}
}

func TestGetTable_unknown(t *testing.T) {
	db := testdb.New(t, fixture)
	if _, err := schema.GetTable(db, "nope"); err == nil {
		t.Fatal("expected error for unknown table")
	}
}

// Finding 1: JSON serialization - nil slices must become empty, not null
func TestGetTable_noForeignKeysSerializesNonNull(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "log")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}

	// These fields must not be null
	checkNotNull := []string{"columns", "primaryKey", "keyColumns", "foreignKeys", "incoming"}
	for _, field := range checkNotNull {
		if m[field] == nil {
			t.Fatalf("field %q must not be null in JSON, got: %s", field, data)
		}
		if v, ok := m[field].([]interface{}); !ok || v == nil {
			t.Fatalf("field %q must be a non-nil array in JSON, got: %v", field, m[field])
		}
	}
}

func TestGetTable_viewSerializesEmptyKeyColumns(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "v_posts")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}

	// keyColumns should be non-nil empty array, not null
	if m["keyColumns"] == nil {
		t.Fatalf("keyColumns must not be null for view, got: %s", data)
	}
	keyColumns, ok := m["keyColumns"].([]interface{})
	if !ok || keyColumns == nil {
		t.Fatalf("keyColumns must be a non-nil array for view, got: %v", m["keyColumns"])
	}
	if len(keyColumns) != 0 {
		t.Fatalf("view should have empty keyColumns, got %d items", len(keyColumns))
	}
}

// Finding 3a: FK referencing non-existent table
func TestGetTable_fkReferencingNonexistentTable(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "fk_broken")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(s.ForeignKeys))
	}
	fk := s.ForeignKeys[0]
	if fk.Table != "nonexistent" {
		t.Fatalf("FK table should be 'nonexistent', got %q", fk.Table)
	}
	// To should stay as [""] when target doesn't exist
	if !reflect.DeepEqual(fk.To, []string{""}) {
		t.Fatalf("FK to should be [\"\"], got %+v", fk.To)
	}
}

// Finding 3b: Composite FK with reordered columns
func TestGetTable_compositeFKReordered(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "reordered_order")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(s.ForeignKeys))
	}
	fk := s.ForeignKeys[0]
	if fk.Table != "tags" {
		t.Fatalf("FK table should be 'tags', got %q", fk.Table)
	}
	// From should be (y, x) in order as they appear in reordered_order
	if !reflect.DeepEqual(fk.From, []string{"y", "x"}) {
		t.Fatalf("FK from should be [y x], got %+v", fk.From)
	}
	// To should be (b, a) as they appear in the REFERENCES clause
	if !reflect.DeepEqual(fk.To, []string{"b", "a"}) {
		t.Fatalf("FK to should be [b a], got %+v", fk.To)
	}
}

// Finding 3c: Composite incoming FK from parent side
func TestGetTable_compositeIncomingFK(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "tags")
	if err != nil {
		t.Fatal(err)
	}

	// Find incoming from tag_notes
	found := false
	var incoming schema.IncomingFK
	for _, f := range s.Incoming {
		if f.Table == "tag_notes" {
			found = true
			incoming = f
			break
		}
	}
	if !found {
		t.Fatal("expected incoming FK from tag_notes to tags")
	}
	if !reflect.DeepEqual(incoming.From, []string{"a", "b"}) {
		t.Fatalf("incoming from should be [a b], got %+v", incoming.From)
	}
	if !reflect.DeepEqual(incoming.To, []string{"a", "b"}) {
		t.Fatalf("incoming to should be [a b], got %+v", incoming.To)
	}
}
