package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Info describes the currently open database.
type Info struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	ReadOnly bool   `json:"readOnly"`
}

// Open opens an existing SQLite file. It never creates a missing file and falls
// back to read-only mode when the file is not writable.
func Open(path string) (*sql.DB, Info, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, Info{}, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, Info{}, fmt.Errorf("não foi possível abrir %s: %w", abs, err)
	}
	if st.IsDir() {
		return nil, Info{}, fmt.Errorf("%s é uma pasta, não um arquivo", abs)
	}

	readOnly := false
	if f, err := os.OpenFile(abs, os.O_RDWR, 0); err != nil {
		readOnly = true
	} else {
		f.Close()
	}
	mode := "rw"
	if readOnly {
		mode = "ro"
	}

	q := url.Values{}
	q.Set("mode", mode)
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	u := url.URL{Scheme: "file", Path: abs, RawQuery: q.Encode()}

	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, Info{}, err
	}
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM sqlite_master`).Scan(&n); err != nil {
		conn.Close()
		return nil, Info{}, fmt.Errorf("%s não é um banco SQLite válido: %w", abs, err)
	}
	return conn, Info{Path: abs, Name: filepath.Base(abs), ReadOnly: readOnly}, nil
}
