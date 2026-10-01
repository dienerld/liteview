package rows_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"liteview/internal/rows"
)

func TestReferences_countsIncomingRows(t *testing.T) {
	db, s := setup(t, "users")
	refs, err := rows.References(db, s, map[string]any{"id": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Table != "posts" || refs[0].Count != 2 ||
		!reflect.DeepEqual(refs[0].From, []string{"user_id"}) || !reflect.DeepEqual(refs[0].To, []string{"id"}) {
		t.Fatalf("refs wrong: %+v", refs)
	}
}

func TestReferences_omitsZeroCounts(t *testing.T) {
	db, s := setup(t, "users")
	refs, err := rows.References(db, s, map[string]any{"id": float64(3)})
	if err != nil || len(refs) != 0 {
		t.Fatalf("expected no refs, got %+v err=%v", refs, err)
	}
}

func TestReferences_incompleteKey(t *testing.T) {
	db, s := setup(t, "users")
	if _, err := rows.References(db, s, map[string]any{}); err == nil {
		t.Fatal("expected error for incomplete key")
	}
}

func TestReferences_emptySliceIsNotNil(t *testing.T) {
	db, s := setup(t, "users")
	refs, err := rows.References(db, s, map[string]any{"id": float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(refs)
	if err != nil || string(b) != "[]" {
		t.Fatalf("refs json = %s err=%v (want [])", b, err)
	}
}

func TestReferences_compositeFK(t *testing.T) {
	db, s := setupExec(t, "tags",
		`CREATE TABLE tags (a INTEGER, b INTEGER, label TEXT, PRIMARY KEY (a, b)) WITHOUT ROWID`,
		`CREATE TABLE tag_notes (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER, FOREIGN KEY (a, b) REFERENCES tags(a, b))`,
		`INSERT INTO tags VALUES (1, 1, 'tag1'), (1, 2, 'tag2'), (2, 1, 'tag3')`,
		`INSERT INTO tag_notes (a, b) VALUES (1, 1), (1, 1), (1, 2)`)

	// Test key {a:1, b:1} should return 2 references
	refs, err := rows.References(db, s, map[string]any{"a": int64(1), "b": int64(1)})
	if err != nil {
		t.Fatalf("composite FK test a=1 b=1: %v", err)
	}
	if len(refs) != 1 || refs[0].Table != "tag_notes" || refs[0].Count != 2 ||
		!reflect.DeepEqual(refs[0].From, []string{"a", "b"}) || !reflect.DeepEqual(refs[0].To, []string{"a", "b"}) {
		t.Fatalf("composite FK a=1 b=1 wrong: %+v", refs)
	}

	// Test key {a:1, b:2} should return 1 reference
	refs, err = rows.References(db, s, map[string]any{"a": int64(1), "b": int64(2)})
	if err != nil {
		t.Fatalf("composite FK test a=1 b=2: %v", err)
	}
	if len(refs) != 1 || refs[0].Count != 1 {
		t.Fatalf("composite FK a=1 b=2 wrong: %+v", refs)
	}

	// Test key {a:2, b:1} should return empty slice (no references)
	refs, err = rows.References(db, s, map[string]any{"a": int64(2), "b": int64(1)})
	if err != nil {
		t.Fatalf("composite FK test a=2 b=1: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("composite FK a=2 b=1 expected empty, got: %+v", refs)
	}
}

func TestReferences_nullFKColumnsNotCounted(t *testing.T) {
	db, s := setupExec(t, "parent",
		`CREATE TABLE parent (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id), data TEXT)`,
		`INSERT INTO parent VALUES (1, 'p1')`,
		`INSERT INTO child (parent_id, data) VALUES (1, 'c1'), (NULL, 'c2'), (1, 'c3')`)

	refs, err := rows.References(db, s, map[string]any{"id": int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	// Should count only 2 (c1 and c3), not the NULL one (c2)
	if len(refs) != 1 || refs[0].Count != 2 {
		t.Fatalf("null FK test wrong: %+v (expected count=2)", refs)
	}
}

func TestReferences_selfReference(t *testing.T) {
	db, s := setupExec(t, "emp",
		`CREATE TABLE emp (id INTEGER PRIMARY KEY, name TEXT, boss_id INTEGER REFERENCES emp(id))`,
		`INSERT INTO emp (name, boss_id) VALUES ('Alice', NULL), ('Bob', 1), ('Carol', 1), ('Dave', 2)`)

	// Boss id=1 (Alice) has 2 reports (Bob, Carol)
	refs, err := rows.References(db, s, map[string]any{"id": int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Table != "emp" || refs[0].Count != 2 {
		t.Fatalf("self-reference alice: %+v (expected count=2)", refs)
	}

	// Boss id=2 (Bob) has 1 report (Dave)
	refs, err = rows.References(db, s, map[string]any{"id": int64(2)})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Count != 1 {
		t.Fatalf("self-reference bob: %+v (expected count=1)", refs)
	}
}

func TestReferences_rowidParentViaUniqueColumn(t *testing.T) {
	db, s := setupExec(t, "p",
		`CREATE TABLE p (code TEXT UNIQUE)`,
		`CREATE TABLE c (id INTEGER PRIMARY KEY, code_ref TEXT REFERENCES p(code))`,
		`INSERT INTO p VALUES ('A'), ('B')`,
		`INSERT INTO c (code_ref) VALUES ('A'), ('A'), ('B')`)

	// Parent has no explicit PK; rowid is implicit. Child references via unique column.
	// Key with rowid=1 should find the references to code='A'
	refs, err := rows.References(db, s, map[string]any{"rowid": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Table != "c" || refs[0].Count != 2 {
		t.Fatalf("rowid parent via unique: %+v (expected count=2)", refs)
	}
}

func TestReferences_oddIdentifiers(t *testing.T) {
	db, s := setupExec(t, "order items",
		`CREATE TABLE "order items" ("select" INTEGER PRIMARY KEY, description TEXT)`,
		`CREATE TABLE "we""ird child" (id INTEGER PRIMARY KEY, "from" INTEGER REFERENCES "order items"("select"))`,
		`INSERT INTO "order items" VALUES (1, 'item1'), (2, 'item2')`,
		`INSERT INTO "we""ird child" VALUES (10, 1), (11, 1), (12, 2)`)

	refs, err := rows.References(db, s, map[string]any{"select": int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Table != `we"ird child` || refs[0].Count != 2 ||
		!reflect.DeepEqual(refs[0].From, []string{"from"}) || !reflect.DeepEqual(refs[0].To, []string{"select"}) {
		t.Fatalf("odd identifiers wrong: %+v", refs)
	}
}

func TestReferences_nonexistentParentRow(t *testing.T) {
	db, s := setup(t, "users")
	// id=999 does not exist in users table
	refs, err := rows.References(db, s, map[string]any{"id": int64(999)})
	if err != nil {
		t.Fatalf("nonexistent row: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("nonexistent row expected empty slice, got: %+v", refs)
	}
}
