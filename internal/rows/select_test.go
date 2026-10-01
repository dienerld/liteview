package rows_test

import (
	"database/sql"
	"reflect"
	"testing"

	"sqliteviewer/internal/rows"
	"sqliteviewer/internal/schema"
	"sqliteviewer/internal/testdb"
)

const fixture = `
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE, name TEXT, active INTEGER DEFAULT 1);
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT DEFAULT 'untitled');
CREATE TABLE log (msg TEXT);
CREATE TABLE "we""ird" ("select" TEXT, "my col" INTEGER, PRIMARY KEY ("select"));
CREATE TABLE types (id INTEGER PRIMARY KEY, big INTEGER, data BLOB, created DATETIME);
CREATE TABLE gen (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) VIRTUAL);
CREATE VIEW v_users AS SELECT id, email FROM users;
INSERT INTO users (email, name) VALUES ('a@x.com','Ana'), ('b@x.com','Bruno'), ('c@x.com','Carla');
INSERT INTO posts (user_id, title) VALUES (1,'p1'), (1,'p2'), (2,'p3');
INSERT INTO log VALUES ('hello');
INSERT INTO "we""ird" VALUES ('x', 1), ('y', NULL);
INSERT INTO types (big, data, created) VALUES (9007199254740993, x'0102', '2024-01-02 03:04:05');
`

func setup(t *testing.T, table string) (*sql.DB, *schema.TableSchema) {
	t.Helper()
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, table)
	if err != nil {
		t.Fatal(err)
	}
	return db, s
}

func TestSelect_pagination(t *testing.T) {
	db, s := setup(t, "users")
	p, err := rows.Select(db, s, rows.Query{Table: "users", Page: 2, PageSize: 2, OrderBy: "id"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 3 || len(p.Rows) != 1 || p.Rows[0].Values["email"] != "c@x.com" {
		t.Fatalf("page wrong: %+v", p)
	}
	if !reflect.DeepEqual(p.Rows[0].Key, map[string]any{"id": int64(3)}) {
		t.Fatalf("key wrong: %+v", p.Rows[0].Key)
	}
}

func TestSelect_orderDesc(t *testing.T) {
	db, s := setup(t, "users")
	p, err := rows.Select(db, s, rows.Query{Table: "users", OrderBy: "email", Desc: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Rows[0].Values["email"] != "c@x.com" {
		t.Fatalf("order wrong: %+v", p.Rows)
	}
}

func TestSelect_rejectsUnknownOrderColumn(t *testing.T) {
	db, s := setup(t, "users")
	for _, bad := range []string{"nope", "id; DROP TABLE users"} {
		if _, err := rows.Select(db, s, rows.Query{Table: "users", OrderBy: bad}); err == nil {
			t.Fatalf("expected error for OrderBy %q", bad)
		}
	}
}

func TestSelect_filterTextColumnsOnly(t *testing.T) {
	db, s := setup(t, "users")
	p, err := rows.Select(db, s, rows.Query{Table: "users", Filter: "bru"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 1 || p.Rows[0].Values["name"] != "Bruno" {
		t.Fatalf("filter wrong: %+v", p)
	}
	p, _ = rows.Select(db, s, rows.Query{Table: "users", Filter: "%"})
	if p.Total != 0 {
		t.Fatalf("%% must be escaped, got total %d", p.Total)
	}
}

func TestSelect_whereConditions(t *testing.T) {
	db, s := setup(t, "posts")
	p, err := rows.Select(db, s, rows.Query{Table: "posts", Where: []rows.Cond{{Column: "user_id", Value: int64(1)}}})
	if err != nil || p.Total != 2 {
		t.Fatalf("where wrong: %+v err=%v", p, err)
	}
	db2, s2 := setup(t, "we\"ird")
	p, err = rows.Select(db2, s2, rows.Query{Table: `we"ird`, Where: []rows.Cond{{Column: "my col", Value: nil}}})
	if err != nil || p.Total != 1 || p.Rows[0].Values["select"] != "y" {
		t.Fatalf("IS NULL wrong: %+v err=%v", p, err)
	}
	if _, err := rows.Select(db, s, rows.Query{Table: "posts", Where: []rows.Cond{{Column: "bogus", Value: 1}}}); err == nil {
		t.Fatal("expected error for unknown where column")
	}
}

func TestSelect_rowidKey(t *testing.T) {
	db, s := setup(t, "log")
	p, err := rows.Select(db, s, rows.Query{Table: "log"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Rows[0].Key, map[string]any{"rowid": int64(1)}) {
		t.Fatalf("rowid key wrong: %+v", p.Rows[0].Key)
	}
	if _, leaked := p.Rows[0].Values["__rowid__"]; leaked {
		t.Fatal("internal rowid alias must not leak into values")
	}
}

func TestSelect_valueFidelity(t *testing.T) {
	db, s := setup(t, "types")
	p, err := rows.Select(db, s, rows.Query{Table: "types"})
	if err != nil {
		t.Fatal(err)
	}
	v := p.Rows[0].Values
	if v["big"] != "9007199254740993" {
		t.Fatalf("big int must be a string, got %#v", v["big"])
	}
	if !reflect.DeepEqual(v["data"], map[string]any{"$blob": 2}) {
		t.Fatalf("blob wrong: %#v", v["data"])
	}
	if v["created"] != "2024-01-02 03:04:05" {
		t.Fatalf("datetime must round-trip as stored, got %#v", v["created"])
	}
}

func TestSelect_weirdIdentifiers(t *testing.T) {
	db, s := setup(t, `we"ird`)
	p, err := rows.Select(db, s, rows.Query{Table: `we"ird`, OrderBy: "my col", Desc: true})
	if err != nil || p.Total != 2 {
		t.Fatalf("weird select failed: %+v err=%v", p, err)
	}
}

func TestSelect_viewAndGeneratedColumn(t *testing.T) {
	db, s := setup(t, "v_users")
	p, err := rows.Select(db, s, rows.Query{Table: "v_users"})
	if err != nil || p.Total != 3 || len(p.Rows[0].Key) != 0 {
		t.Fatalf("view select wrong: %+v err=%v", p, err)
	}
	db2, s2 := setup(t, "gen")
	if _, err := db2.Exec(`INSERT INTO gen (a) VALUES (4)`); err != nil {
		t.Fatal(err)
	}
	p, err = rows.Select(db2, s2, rows.Query{Table: "gen"})
	if err != nil || p.Rows[0].Values["b"] != int64(8) {
		t.Fatalf("generated column wrong: %+v err=%v", p, err)
	}
}

func TestSelect_clampsPageSize(t *testing.T) {
	db, s := setup(t, "users")
	p, err := rows.Select(db, s, rows.Query{Table: "users", Page: 0, PageSize: 100000})
	if err != nil || p.PageSize != 500 || p.Page != 1 {
		t.Fatalf("clamp wrong: %+v err=%v", p, err)
	}
}
