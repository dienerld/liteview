package rows_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"sqliteviewer/internal/rows"
	"sqliteviewer/internal/schema"
)

func TestInsert_returnsKeyAndAppliesDefaults(t *testing.T) {
	db, s := setup(t, "posts")
	key, err := rows.Insert(db, s, map[string]any{"user_id": float64(2)}) // JSON numbers arrive as float64
	if err != nil {
		t.Fatal(err)
	}
	if key["id"] != int64(4) {
		t.Fatalf("key wrong: %#v", key)
	}
	p, _ := rows.Select(db, s, rows.Query{Table: "posts", Where: []rows.Cond{{Column: "id", Value: int64(4)}}})
	if p.Rows[0].Values["title"] != "untitled" {
		t.Fatalf("default not applied: %+v", p.Rows[0].Values)
	}
}

func TestInsert_defaultValuesWhenNothingProvided(t *testing.T) {
	db, s := setup(t, "log")
	key, err := rows.Insert(db, s, map[string]any{})
	if err != nil || key["rowid"] != int64(2) {
		t.Fatalf("DEFAULT VALUES insert wrong: key=%#v err=%v", key, err)
	}
}

func TestInsert_explicitNullVsEmptyString(t *testing.T) {
	db, s := setup(t, "users")
	if _, err := rows.Insert(db, s, map[string]any{"email": "n@x.com", "name": nil}); err != nil {
		t.Fatal(err)
	}
	if _, err := rows.Insert(db, s, map[string]any{"email": "e@x.com", "name": ""}); err != nil {
		t.Fatal(err)
	}
	p, _ := rows.Select(db, s, rows.Query{Table: "users", Where: []rows.Cond{{Column: "email", Value: "n@x.com"}}})
	if p.Rows[0].Values["name"] != nil {
		t.Fatalf("expected NULL, got %#v", p.Rows[0].Values["name"])
	}
	p, _ = rows.Select(db, s, rows.Query{Table: "users", Where: []rows.Cond{{Column: "email", Value: "e@x.com"}}})
	if p.Rows[0].Values["name"] != "" {
		t.Fatalf("expected empty string, got %#v", p.Rows[0].Values["name"])
	}
}

func TestInsert_constraintErrors(t *testing.T) {
	db, s := setup(t, "users")
	_, err := rows.Insert(db, s, map[string]any{"name": "sem email"})
	var ce *rows.ConstraintError
	if !errors.As(err, &ce) || ce.Kind != "NOT NULL" || !reflect.DeepEqual(ce.Columns, []string{"email"}) {
		t.Fatalf("NOT NULL error wrong: %#v", err)
	}
	_, err = rows.Insert(db, s, map[string]any{"email": "a@x.com"})
	if !errors.As(err, &ce) || ce.Kind != "UNIQUE" || !reflect.DeepEqual(ce.Columns, []string{"email"}) {
		t.Fatalf("UNIQUE error wrong: %#v", err)
	}
	dbp, sp := setup(t, "posts")
	_, err = rows.Insert(dbp, sp, map[string]any{"user_id": float64(999)})
	if !errors.As(err, &ce) || ce.Kind != "FOREIGN KEY" {
		t.Fatalf("FOREIGN KEY error wrong: %#v", err)
	}
	if ce.Msg == "" {
		t.Fatal("constraint error needs a readable message")
	}
}

func TestInsert_rejectsUnknownAndGeneratedColumns(t *testing.T) {
	db, s := setup(t, "users")
	if _, err := rows.Insert(db, s, map[string]any{"email": "z@x.com", "bogus": 1}); err == nil {
		t.Fatal("expected error for unknown column")
	}
	dbg, sg := setup(t, "gen")
	if _, err := rows.Insert(dbg, sg, map[string]any{"a": 1, "b": 2}); err == nil {
		t.Fatal("expected error for generated column")
	}
}

func TestInsert_weirdIdentifiers(t *testing.T) {
	db, s := setup(t, `we"ird`)
	key, err := rows.Insert(db, s, map[string]any{"select": "z", "my col": float64(7)})
	if err != nil || key["select"] != "z" {
		t.Fatalf("weird insert wrong: key=%#v err=%v", key, err)
	}
}

func TestUpdate_changesOnlyGivenColumns(t *testing.T) {
	db, s := setup(t, "users")
	if err := rows.Update(db, s, map[string]any{"id": float64(1)}, map[string]any{"name": "Ana Maria"}); err != nil {
		t.Fatal(err)
	}
	p, _ := rows.Select(db, s, rows.Query{Table: "users", Where: []rows.Cond{{Column: "id", Value: int64(1)}}})
	if p.Rows[0].Values["name"] != "Ana Maria" || p.Rows[0].Values["email"] != "a@x.com" {
		t.Fatalf("update wrong: %+v", p.Rows[0].Values)
	}
}

func TestUpdate_rowidTableAndMissingRow(t *testing.T) {
	db, s := setup(t, "log")
	if err := rows.Update(db, s, map[string]any{"rowid": float64(1)}, map[string]any{"msg": "bye"}); err != nil {
		t.Fatal(err)
	}
	if err := rows.Update(db, s, map[string]any{"rowid": float64(99)}, map[string]any{"msg": "x"}); err == nil {
		t.Fatal("expected error when row does not exist")
	}
	if err := rows.Update(db, s, map[string]any{}, map[string]any{"msg": "x"}); err == nil {
		t.Fatal("expected error for incomplete key")
	}
}

func TestDelete_andFKBlocksIt(t *testing.T) {
	db, s := setup(t, "users")
	var ce *rows.ConstraintError
	err := rows.Delete(db, s, map[string]any{"id": float64(1)}) // user 1 has posts
	if !errors.As(err, &ce) || ce.Kind != "FOREIGN KEY" {
		t.Fatalf("expected FK error, got %#v", err)
	}
	if err := rows.Delete(db, s, map[string]any{"id": float64(3)}); err != nil {
		t.Fatal(err)
	}
	if err := rows.Delete(db, s, map[string]any{"id": float64(3)}); err == nil {
		t.Fatal("deleting a missing row must error")
	}
}

func TestMutations_viewsAreReadOnly(t *testing.T) {
	db, s := setup(t, "v_users")
	if _, err := rows.Insert(db, s, map[string]any{"email": "x"}); !errors.Is(err, rows.ErrReadOnly) {
		t.Fatalf("insert on view: %v", err)
	}
	if err := rows.Update(db, s, map[string]any{"id": 1}, map[string]any{"email": "x"}); !errors.Is(err, rows.ErrReadOnly) {
		t.Fatalf("update on view: %v", err)
	}
	if err := rows.Delete(db, s, map[string]any{"id": 1}); !errors.Is(err, rows.ErrReadOnly) {
		t.Fatalf("delete on view: %v", err)
	}
}

var _ = schema.TableSchema{} // keep import used if helpers move

func TestConstraintError_primaryKeyAndJSON(t *testing.T) {
	db, s := setup(t, "users")
	var ce *rows.ConstraintError
	_, err := rows.Insert(db, s, map[string]any{"id": float64(1), "email": "new@x.com"})
	if !errors.As(err, &ce) || ce.Kind != "PRIMARY KEY" || !reflect.DeepEqual(ce.Columns, []string{"id"}) {
		t.Fatalf("PRIMARY KEY error wrong: %#v", err)
	}
	dbp, sp := setup(t, "posts")
	_, err = rows.Insert(dbp, sp, map[string]any{"user_id": float64(999)})
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	b, _ := json.Marshal(ce)
	if !strings.Contains(string(b), `"columns":[]`) {
		t.Fatalf("columns must marshal as [], got %s", b)
	}
}

func TestConstraintError_compositeUniqueAndCheck(t *testing.T) {
	db, s := setupExec(t, "cu",
		`CREATE TABLE cu (id INTEGER PRIMARY KEY, a INT, b INT, c INT CHECK (c > 0), UNIQUE (a, b))`,
		`INSERT INTO cu (a, b, c) VALUES (1, 1, 1)`)
	var ce *rows.ConstraintError
	_, err := rows.Insert(db, s, map[string]any{"a": float64(1), "b": float64(1), "c": float64(1)})
	if !errors.As(err, &ce) || ce.Kind != "UNIQUE" || !reflect.DeepEqual(ce.Columns, []string{"a", "b"}) {
		t.Fatalf("composite UNIQUE wrong: %#v", err)
	}
	_, err = rows.Insert(db, s, map[string]any{"a": float64(2), "b": float64(2), "c": float64(0)})
	if !errors.As(err, &ce) || ce.Kind != "CHECK" || len(ce.Columns) != 0 {
		t.Fatalf("CHECK wrong: %#v", err)
	}
}
