package rows_test

import (
	"database/sql"
	"encoding/json"
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

// setupExec runs extra statements on the fixture before reading the schema.
func setupExec(t *testing.T, table string, stmts ...string) (*sql.DB, *schema.TableSchema) {
	t.Helper()
	db := testdb.New(t, fixture)
	for _, st := range stmts {
		if _, err := db.Exec(st); err != nil {
			t.Fatal(err)
		}
	}
	s, err := schema.GetTable(db, table)
	if err != nil {
		t.Fatal(err)
	}
	return db, s
}

// collect pages through the whole result and returns the values of col in order.
func collect(t *testing.T, db *sql.DB, s *schema.TableSchema, q rows.Query, col string) []any {
	t.Helper()
	q.PageSize = 2
	var got []any
	for page := 1; page < 20; page++ {
		q.Page = page
		p, err := rows.Select(db, s, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Rows) == 0 {
			return got
		}
		for _, r := range p.Rows {
			got = append(got, r.Values[col])
		}
	}
	t.Fatal("too many pages")
	return nil
}

func TestSelect_tieBreakerRowidTable(t *testing.T) {
	db, s := setupExec(t, "log",
		`INSERT INTO log VALUES ('dup'), ('dup'), ('dup'), ('zzz')`)
	// rowids: hello=1, dup=2,3,4, zzz=5
	q := rows.Query{Table: "log", OrderBy: "msg"}
	var ids []any
	q.PageSize = 2
	for page := 1; page <= 3; page++ {
		q.Page = page
		p, err := rows.Select(db, s, q)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range p.Rows {
			ids = append(ids, r.Key["rowid"])
		}
	}
	want := []any{int64(2), int64(3), int64(4), int64(1), int64(5)}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("rowid pages = %v, want %v", ids, want)
	}
}

func TestSelect_tieBreakerKeyedTable(t *testing.T) {
	db, s := setupExec(t, "users",
		`INSERT INTO users (email, name) VALUES ('d@x.com','Ana'), ('e@x.com','Ana'), ('f@x.com','Ana')`)
	// ids: Ana=1,4,5,6 Bruno=2 Carla=3
	got := collect(t, db, s, rows.Query{Table: "users", OrderBy: "name"}, "email")
	want := []any{"a@x.com", "d@x.com", "e@x.com", "f@x.com", "b@x.com", "c@x.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("asc = %v, want %v", got, want)
	}
	got = collect(t, db, s, rows.Query{Table: "users", OrderBy: "name", Desc: true}, "email")
	want = []any{"c@x.com", "b@x.com", "a@x.com", "d@x.com", "e@x.com", "f@x.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("desc = %v, want %v", got, want)
	}
}

func TestSelect_defaultOrderByKey(t *testing.T) {
	db, s := setup(t, "users")
	got := collect(t, db, s, rows.Query{Table: "users"}, "email")
	if !reflect.DeepEqual(got, []any{"a@x.com", "b@x.com", "c@x.com"}) {
		t.Fatalf("users default order = %v", got)
	}

	db, s = setupExec(t, "kt",
		`CREATE TABLE kt (k TEXT PRIMARY KEY, v INTEGER)`,
		`INSERT INTO kt VALUES ('z',1), ('a',2), ('m',3)`)
	got = collect(t, db, s, rows.Query{Table: "kt"}, "k")
	if !reflect.DeepEqual(got, []any{"a", "m", "z"}) {
		t.Fatalf("text-pk default order = %v", got)
	}

	db, s = setupExec(t, "wr",
		`CREATE TABLE wr (a INTEGER, b INTEGER, PRIMARY KEY (b, a)) WITHOUT ROWID`,
		`INSERT INTO wr VALUES (2,2), (1,2), (2,1), (1,1)`)
	p, err := rows.Select(db, s, rows.Query{Table: "wr"})
	if err != nil {
		t.Fatal(err)
	}
	var pairs [][2]any
	for _, r := range p.Rows {
		pairs = append(pairs, [2]any{r.Values["b"], r.Values["a"]})
	}
	want := [][2]any{{int64(1), int64(1)}, {int64(1), int64(2)}, {int64(2), int64(1)}, {int64(2), int64(2)}}
	if !reflect.DeepEqual(pairs, want) {
		t.Fatalf("composite default order = %v, want %v", pairs, want)
	}
}

func TestSelect_emptyResultIsEmptyArray(t *testing.T) {
	db, s := setup(t, "users")
	p, err := rows.Select(db, s, rows.Query{Table: "users", Filter: "zzz-no-match"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p.Rows)
	if err != nil || string(b) != "[]" {
		t.Fatalf("rows json = %s err=%v", b, err)
	}
}

func TestSelect_filterEscapesLikeWildcards(t *testing.T) {
	db, s := setupExec(t, "users",
		`INSERT INTO users (email, name) VALUES ('u1@x.com','a_c'), ('u2@x.com','abc'), ('u3@x.com','p\q'), ('u4@x.com','pxq')`)
	p, err := rows.Select(db, s, rows.Query{Table: "users", Filter: "a_c"})
	if err != nil || p.Total != 1 || p.Rows[0].Values["name"] != "a_c" {
		t.Fatalf("underscore must be literal: %+v err=%v", p, err)
	}
	p, err = rows.Select(db, s, rows.Query{Table: "users", Filter: `p\q`})
	if err != nil || p.Total != 1 || p.Rows[0].Values["name"] != `p\q` {
		t.Fatalf("backslash must be literal: %+v err=%v", p, err)
	}
}

func TestSelect_filterSkipsNonTextColumns(t *testing.T) {
	db, s := setup(t, "users") // active INTEGER DEFAULT 1 -> every row has active=1
	p, err := rows.Select(db, s, rows.Query{Table: "users", Filter: "1"})
	if err != nil || p.Total != 0 {
		t.Fatalf("integer column must not be searched: total=%d err=%v", p.Total, err)
	}
}
