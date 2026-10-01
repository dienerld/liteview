package rows

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sqliteviewer/internal/schema"
)

const maxSafeInt = int64(1) << 53

type Cond struct {
	Column string `json:"column"`
	Value  any    `json:"value"`
}

type Query struct {
	Table    string `json:"table"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	OrderBy  string `json:"orderBy"`
	Desc     bool   `json:"desc"`
	Filter   string `json:"filter"`
	Where    []Cond `json:"where"`
}

type Row struct {
	Key    map[string]any `json:"key"`
	Values map[string]any `json:"values"`
}

type Page struct {
	Rows     []Row `json:"rows"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// quote returns id as a double-quoted SQL identifier.
func quote(id string) string {
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

// normalize converts driver values into JSON-safe values for the frontend.
func normalize(v any) any {
	switch x := v.(type) {
	case []byte:
		return map[string]any{"$blob": len(x)}
	case int64:
		if x > maxSafeInt || x < -maxSafeInt {
			return strconv.FormatInt(x, 10)
		}
		return x
	case time.Time:
		return x.Format(time.RFC3339Nano)
	}
	return v
}

func isDateLike(decl string) bool {
	d := strings.ToUpper(decl)
	return strings.Contains(d, "DATE") || strings.Contains(d, "TIME")
}

func isTextColumn(decl string) bool {
	d := strings.ToUpper(decl)
	return d == "" || strings.Contains(d, "CHAR") || strings.Contains(d, "CLOB") || strings.Contains(d, "TEXT")
}

// selectExpr reads date-like columns as raw text so the driver does not
// reinterpret them as time.Time.
func selectExpr(c schema.Column) string {
	if isDateLike(c.Type) {
		return "CAST(" + quote(c.Name) + " AS TEXT) AS " + quote(c.Name)
	}
	return quote(c.Name)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return strings.ReplaceAll(s, `_`, `\_`)
}

func hasColumn(s *schema.TableSchema, name string) bool {
	for _, c := range s.Columns {
		if c.Name == name {
			return true
		}
	}
	return false
}

func buildWhere(s *schema.TableSchema, q Query) (string, []any, error) {
	var parts []string
	var args []any
	for _, c := range q.Where {
		if !hasColumn(s, c.Column) {
			return "", nil, fmt.Errorf("coluna desconhecida: %q", c.Column)
		}
		if c.Value == nil {
			parts = append(parts, quote(c.Column)+" IS NULL")
		} else {
			parts = append(parts, quote(c.Column)+" = ?")
			args = append(args, c.Value)
		}
	}
	if q.Filter != "" {
		var ors []string
		for _, c := range s.Columns {
			if isTextColumn(c.Type) {
				ors = append(ors, "CAST("+quote(c.Name)+` AS TEXT) LIKE ? ESCAPE '\'`)
				args = append(args, "%"+escapeLike(q.Filter)+"%")
			}
		}
		if len(ors) == 0 {
			parts = append(parts, "0")
		} else {
			parts = append(parts, "("+strings.Join(ors, " OR ")+")")
		}
	}
	if len(parts) == 0 {
		return "", nil, nil
	}
	return " WHERE " + strings.Join(parts, " AND "), args, nil
}

// buildOrder returns the ORDER BY clause. The key (rowid or primary key
// columns) is always appended as a tie-breaker so pagination is stable; with no
// OrderBy it is the whole ordering. Views have no key and stay unordered.
func buildOrder(s *schema.TableSchema, q Query) (string, error) {
	var terms []string
	if q.OrderBy != "" {
		if !hasColumn(s, q.OrderBy) {
			return "", fmt.Errorf("coluna desconhecida: %q", q.OrderBy)
		}
		term := quote(q.OrderBy)
		if q.Desc {
			term += " DESC"
		}
		terms = append(terms, term)
	}
	if s.UsesRowID {
		terms = append(terms, "rowid")
	} else {
		for _, k := range s.KeyColumns {
			terms = append(terms, quote(k))
		}
	}
	if len(terms) == 0 {
		return "", nil
	}
	return " ORDER BY " + strings.Join(terms, ", "), nil
}

// Select reads one page of rows with optional ordering, filter and exact conditions.
func Select(db *sql.DB, s *schema.TableSchema, q Query) (*Page, error) {
	size := q.PageSize
	if size < 1 {
		size = 50
	}
	if size > 500 {
		size = 500
	}
	page := q.Page
	if page < 1 {
		page = 1
	}

	where, args, err := buildWhere(s, q)
	if err != nil {
		return nil, err
	}

	var total int64
	if err := db.QueryRow("SELECT COUNT(*) FROM "+quote(s.Name)+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	exprs := make([]string, 0, len(s.Columns)+1)
	for _, c := range s.Columns {
		exprs = append(exprs, selectExpr(c))
	}
	if s.UsesRowID {
		exprs = append(exprs, `rowid AS "__rowid__"`)
	}

	order, err := buildOrder(s, q)
	if err != nil {
		return nil, err
	}

	query := "SELECT " + strings.Join(exprs, ", ") + " FROM " + quote(s.Name) + where + order + " LIMIT ? OFFSET ?"
	qargs := append(append([]any{}, args...), size, (page-1)*size)
	rs, err := db.Query(query, qargs...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	out := &Page{Rows: []Row{}, Total: total, Page: page, PageSize: size}
	n := len(exprs)
	for rs.Next() {
		vals := make([]any, n)
		ptrs := make([]any, n)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := Row{Key: map[string]any{}, Values: make(map[string]any, len(s.Columns))}
		for i, c := range s.Columns {
			row.Values[c.Name] = normalize(vals[i])
		}
		if s.UsesRowID {
			row.Key["rowid"] = normalize(vals[n-1])
		} else {
			for _, k := range s.KeyColumns {
				row.Key[k] = row.Values[k]
			}
		}
		out.Rows = append(out.Rows, row)
	}
	return out, rs.Err()
}
