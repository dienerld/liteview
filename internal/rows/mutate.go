package rows

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"liteview/internal/schema"
)

var ErrReadOnly = errors.New("esta tabela é somente leitura")

// ConstraintError is a violated constraint, with the columns involved when SQLite reports them.
type ConstraintError struct {
	Kind    string   `json:"kind"`
	Columns []string `json:"columns"`
	Msg     string   `json:"message"`
}

func (e *ConstraintError) Error() string { return e.Msg }

// constraintRe matches the driver text, e.g.
// "constraint failed: UNIQUE constraint failed: users.email (2067)".
var constraintRe = regexp.MustCompile(`(NOT NULL|UNIQUE|PRIMARY KEY|CHECK|FOREIGN KEY) constraint failed(?:: (.*?))?(?: \(\d+\))?$`)

// sqliteCoder is implemented by the modernc driver error; the extended code
// distinguishes a primary key violation (1555), which the message reports as UNIQUE.
type sqliteCoder interface{ Code() int }

const extConstraintPrimaryKey = 1555

func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	m := constraintRe.FindStringSubmatch(err.Error())
	if m == nil {
		return err
	}
	ce := &ConstraintError{Kind: m[1], Columns: []string{}}
	var coder sqliteCoder
	if errors.As(err, &coder) && coder.Code() == extConstraintPrimaryKey {
		ce.Kind = "PRIMARY KEY"
	}
	if m[1] == "NOT NULL" || m[1] == "UNIQUE" || m[1] == "PRIMARY KEY" {
		for _, part := range strings.Split(strings.TrimSpace(m[2]), ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if i := strings.LastIndex(part, "."); i >= 0 {
				part = part[i+1:]
			}
			ce.Columns = append(ce.Columns, part)
		}
	}
	cols := strings.Join(ce.Columns, ", ")
	switch ce.Kind {
	case "NOT NULL":
		ce.Msg = cols + " é obrigatório"
	case "UNIQUE", "PRIMARY KEY":
		ce.Msg = "já existe um registro com este valor em: " + cols
	case "FOREIGN KEY":
		ce.Msg = "o registro referenciado não existe, ou ainda há registros que dependem deste"
	default:
		ce.Msg = "uma regra CHECK da tabela foi violada"
	}
	return ce
}

// assignable validates the columns of values and returns them in a stable order.
func assignable(s *schema.TableSchema, values map[string]any) ([]string, []any, error) {
	cols := make([]string, 0, len(values))
	for name := range values {
		if !hasColumn(s, name) {
			return nil, nil, fmt.Errorf("coluna desconhecida: %q", name)
		}
		for _, c := range s.Columns {
			if c.Name == name && c.Generated {
				return nil, nil, fmt.Errorf("a coluna %q é gerada e não pode ser alterada", name)
			}
		}
		cols = append(cols, name)
	}
	sort.Strings(cols)
	args := make([]any, len(cols))
	for i, c := range cols {
		args[i] = values[c]
	}
	return cols, args, nil
}

func joinQuoted(cols []string) string {
	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = quote(c)
	}
	return strings.Join(q, ", ")
}

func keyList(s *schema.TableSchema) string {
	if s.UsesRowID {
		return "rowid"
	}
	return joinQuoted(s.KeyColumns)
}

// whereKey builds the WHERE clause that identifies exactly one row.
func whereKey(s *schema.TableSchema, key map[string]any) (string, []any, error) {
	if len(s.KeyColumns) == 0 {
		return "", nil, ErrReadOnly
	}
	var parts []string
	var args []any
	for _, k := range s.KeyColumns {
		v, ok := key[k]
		if !ok {
			return "", nil, fmt.Errorf("chave incompleta: falta %q", k)
		}
		col := quote(k)
		if s.UsesRowID {
			col = "rowid"
		}
		if v == nil {
			parts = append(parts, col+" IS NULL")
		} else {
			parts = append(parts, col+" = ?")
			args = append(args, v)
		}
	}
	return strings.Join(parts, " AND "), args, nil
}

func Insert(db *sql.DB, s *schema.TableSchema, values map[string]any) (map[string]any, error) {
	if s.ReadOnly {
		return nil, ErrReadOnly
	}
	cols, args, err := assignable(s, values)
	if err != nil {
		return nil, err
	}
	var q string
	if len(cols) == 0 {
		q = "INSERT INTO " + quote(s.Name) + " DEFAULT VALUES RETURNING " + keyList(s)
	} else {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
		q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s", quote(s.Name), joinQuoted(cols), ph, keyList(s))
	}
	dest := make([]any, len(s.KeyColumns))
	ptrs := make([]any, len(dest))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	if err := db.QueryRow(q, args...).Scan(ptrs...); err != nil {
		return nil, wrapErr(err)
	}
	key := make(map[string]any, len(dest))
	for i, k := range s.KeyColumns {
		key[k] = normalize(dest[i])
	}
	return key, nil
}

func Update(db *sql.DB, s *schema.TableSchema, key, values map[string]any) error {
	if s.ReadOnly {
		return ErrReadOnly
	}
	cols, args, err := assignable(s, values)
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return errors.New("nenhuma alteração para salvar")
	}
	where, kargs, err := whereKey(s, key)
	if err != nil {
		return err
	}
	sets := make([]string, len(cols))
	for i, c := range cols {
		sets[i] = quote(c) + " = ?"
	}
	res, err := db.Exec("UPDATE "+quote(s.Name)+" SET "+strings.Join(sets, ", ")+" WHERE "+where, append(args, kargs...)...)
	if err != nil {
		return wrapErr(err)
	}
	return expectOne(res)
}

func Delete(db *sql.DB, s *schema.TableSchema, key map[string]any) error {
	if s.ReadOnly {
		return ErrReadOnly
	}
	where, kargs, err := whereKey(s, key)
	if err != nil {
		return err
	}
	res, err := db.Exec("DELETE FROM "+quote(s.Name)+" WHERE "+where, kargs...)
	if err != nil {
		return wrapErr(err)
	}
	return expectOne(res)
}

func expectOne(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("registro não encontrado (pode ter sido alterado ou removido)")
	}
	return nil
}
