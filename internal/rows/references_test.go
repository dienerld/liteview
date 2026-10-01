package rows_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"sqliteviewer/internal/rows"
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
