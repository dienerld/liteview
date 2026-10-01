package schema_test

import (
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
	if len(s.ForeignKeys) != 1 || s.ForeignKeys[0].Table != "users" {
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
	fk := s.ForeignKeys[0]
	if fk.Table != "users" || !reflect.DeepEqual(fk.To, []string{"id"}) {
		t.Fatalf("implicit target not resolved: %+v", fk)
	}
	users, _ := schema.GetTable(db, "users")
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
