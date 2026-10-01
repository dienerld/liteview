package rows

import (
	"database/sql"
	"fmt"
	"strings"

	"liteview/internal/schema"
)

type RefCount struct {
	Table string   `json:"table"`
	From  []string `json:"from"`
	To    []string `json:"to"`
	Count int64    `json:"count"`
}

// References counts, per referencing table, the rows that point at the row identified by key.
func References(db *sql.DB, s *schema.TableSchema, key map[string]any) ([]RefCount, error) {
	where, kargs, err := whereKey(s, key)
	if err != nil {
		return nil, err
	}
	out := []RefCount{}
	for _, in := range s.Incoming {
		conds := make([]string, len(in.From))
		var args []any
		for i := range in.From {
			conds[i] = fmt.Sprintf("%s = (SELECT %s FROM %s WHERE %s)",
				quote(in.From[i]), quote(in.To[i]), quote(s.Name), where)
			args = append(args, kargs...)
		}
		var n int64
		q := "SELECT COUNT(*) FROM " + quote(in.Table) + " WHERE " + strings.Join(conds, " AND ")
		if err := db.QueryRow(q, args...).Scan(&n); err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, RefCount{Table: in.Table, From: in.From, To: in.To, Count: n})
		}
	}
	return out, nil
}
