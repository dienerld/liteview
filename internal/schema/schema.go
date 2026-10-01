package schema

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type TableInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Column struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	NotNull   bool    `json:"notNull"`
	Default   *string `json:"default"`
	PK        int     `json:"pk"`
	Generated bool    `json:"generated"`
}

type ForeignKey struct {
	Table    string   `json:"table"`
	From     []string `json:"from"`
	To       []string `json:"to"`
	OnUpdate string   `json:"onUpdate"`
	OnDelete string   `json:"onDelete"`
}

type IncomingFK struct {
	Table string   `json:"table"`
	From  []string `json:"from"`
	To    []string `json:"to"`
}

type TableSchema struct {
	Name        string       `json:"name"`
	Kind        string       `json:"kind"`
	ReadOnly    bool         `json:"readOnly"`
	Columns     []Column     `json:"columns"`
	PrimaryKey  []string     `json:"primaryKey"`
	UsesRowID   bool         `json:"usesRowId"`
	KeyColumns  []string     `json:"keyColumns"`
	ForeignKeys []ForeignKey `json:"foreignKeys"`
	Incoming    []IncomingFK `json:"incoming"`
}

// ListTables returns user tables followed by views, hiding sqlite_ internals.
func ListTables(db *sql.DB) ([]TableInfo, error) {
	rs, err := db.Query(`SELECT name, type FROM sqlite_master
		WHERE type IN ('table','view') AND name NOT LIKE 'sqlite\_%' ESCAPE '\'
		ORDER BY type, name`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []TableInfo{} // Initialize to non-nil empty slice
	for rs.Next() {
		var ti TableInfo
		if err := rs.Scan(&ti.Name, &ti.Kind); err != nil {
			return nil, err
		}
		out = append(out, ti)
	}
	return out, rs.Err()
}

// GetTable reads the full schema of one table or view, including incoming FKs.
func GetTable(db *sql.DB, name string) (*TableSchema, error) {
	var kind string
	err := db.QueryRow(`SELECT type FROM sqlite_master WHERE type IN ('table','view') AND name = ?`, name).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("tabela %q não encontrada", name)
	}
	if err != nil {
		return nil, err
	}

	cols, err := columns(db, name)
	if err != nil {
		return nil, err
	}
	// Initialize all slices to non-nil empty slices
	if cols == nil {
		cols = []Column{}
	}
	s := &TableSchema{
		Name:        name,
		Kind:        kind,
		Columns:     cols,
		PrimaryKey:  []string{},
		KeyColumns:  []string{},
		ForeignKeys: []ForeignKey{},
		Incoming:    []IncomingFK{},
	}

	type pkCol struct {
		pos  int
		name string
	}
	var pks []pkCol
	for _, c := range cols {
		if c.PK > 0 {
			pks = append(pks, pkCol{c.PK, c.Name})
		}
	}
	for i := 1; i <= len(pks); i++ {
		for _, p := range pks {
			if p.pos == i {
				s.PrimaryKey = append(s.PrimaryKey, p.name)
			}
		}
	}

	hasRowID := false
	if kind == "table" {
		var withoutRowID int
		err := db.QueryRow(`SELECT wr FROM pragma_table_list WHERE name = ? AND schema = 'main'`, name).Scan(&withoutRowID)
		if err != nil {
			return nil, err
		}
		hasRowID = withoutRowID == 0
	}
	switch {
	case len(s.PrimaryKey) > 0:
		s.KeyColumns = s.PrimaryKey
	case hasRowID:
		s.KeyColumns = []string{"rowid"}
		s.UsesRowID = true
	}
	s.ReadOnly = kind == "view" || len(s.KeyColumns) == 0

	if s.ForeignKeys, err = foreignKeys(db, name); err != nil {
		return nil, err
	}
	if s.ForeignKeys == nil {
		s.ForeignKeys = []ForeignKey{}
	}
	if s.Incoming, err = incoming(db, name); err != nil {
		return nil, err
	}
	if s.Incoming == nil {
		s.Incoming = []IncomingFK{}
	}
	return s, nil
}

func columns(db *sql.DB, table string) ([]Column, error) {
	rs, err := db.Query(`SELECT name, type, "notnull", dflt_value, pk, hidden FROM pragma_table_xinfo(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []Column{} // Initialize to non-nil empty slice
	for rs.Next() {
		var c Column
		var dflt sql.NullString
		var nn, hidden int
		if err := rs.Scan(&c.Name, &c.Type, &nn, &dflt, &c.PK, &hidden); err != nil {
			return nil, err
		}
		if hidden == 1 { // hidden column of a virtual table
			continue
		}
		c.NotNull = nn != 0
		c.Generated = hidden == 2 || hidden == 3
		if dflt.Valid {
			d := dflt.String
			c.Default = &d
		}
		out = append(out, c)
	}
	return out, rs.Err()
}

func primaryKey(db *sql.DB, table string) ([]string, error) {
	rs, err := db.Query(`SELECT name FROM pragma_table_info(?) WHERE pk > 0 ORDER BY pk`, table)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []string{} // Initialize to non-nil empty slice
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rs.Err()
}

// foreignKeys returns outgoing FKs, resolving an omitted target column
// (REFERENCES parent) to the parent's primary key.
func foreignKeys(db *sql.DB, table string) ([]ForeignKey, error) {
	rs, err := db.Query(`SELECT id, "table", "from", "to", on_update, on_delete
		FROM pragma_foreign_key_list(?) ORDER BY id, seq`, table)
	if err != nil {
		return nil, err
	}
	fks := []ForeignKey{} // Initialize to non-nil empty slice
	byID := map[int]int{}
	for rs.Next() {
		var id int
		var ref, from, onU, onD string
		var to sql.NullString
		if err := rs.Scan(&id, &ref, &from, &to, &onU, &onD); err != nil {
			rs.Close()
			return nil, err
		}
		i, ok := byID[id]
		if !ok {
			fks = append(fks, ForeignKey{
				Table:    ref,
				From:     []string{},
				To:       []string{},
				OnUpdate: onU,
				OnDelete: onD,
			})
			i = len(fks) - 1
			byID[id] = i
		}
		fks[i].From = append(fks[i].From, from)
		fks[i].To = append(fks[i].To, to.String)
	}
	if err := rs.Err(); err != nil {
		rs.Close()
		return nil, err
	}
	rs.Close() // release the connection before nested queries

	for i := range fks {
		for j, t := range fks[i].To {
			if t != "" {
				continue
			}
			pk, err := primaryKey(db, fks[i].Table)
			if err != nil {
				// If the target table doesn't exist, leave the To[j] as ""
				continue
			}
			if j < len(pk) {
				fks[i].To[j] = pk[j]
			}
		}
	}
	return fks, nil
}

func incoming(db *sql.DB, name string) ([]IncomingFK, error) {
	tables, err := ListTables(db)
	if err != nil {
		return nil, err
	}
	out := []IncomingFK{} // Initialize to non-nil empty slice
	for _, ti := range tables {
		if ti.Kind != "table" {
			continue
		}
		fks, err := foreignKeys(db, ti.Name)
		if err != nil {
			return nil, err
		}
		for _, fk := range fks {
			if strings.EqualFold(fk.Table, name) {
				incoming := IncomingFK{
					Table: ti.Name,
					From:  fk.From,
					To:    fk.To,
				}
				// Ensure slices are non-nil
				if incoming.From == nil {
					incoming.From = []string{}
				}
				if incoming.To == nil {
					incoming.To = []string{}
				}
				out = append(out, incoming)
			}
		}
	}
	return out, nil
}
