# SQLite Viewer v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** App desktop (Wails v3 + Vue + shadcn-vue) para visualizar e editar bancos SQLite: tabelas, estrutura, navegação por FKs, insert/update/delete por formulário em painel lateral.

**Architecture:** Backend Go em pacotes pequenos (`db`, `schema`, `rows`, `recents`, `viewer`) sob `internal/`. O `viewer.Service` é o único service Wails; expõe métodos tipados e o SQL é montado no backend (identificadores validados contra o schema, valores sempre por parâmetro). O frontend Vue só chama os bindings gerados, por uma camada fina (`lib/api.ts`).

**Tech Stack:** Go ≥ 1.24, Wails v3 (`v3.0.0-beta.26`), `modernc.org/sqlite`, Vue 3 + TypeScript + Vite, Tailwind v4, shadcn-vue, `@tanstack/vue-table`, Pinia, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-01-sqlite-viewer-design.md`

## Global Constraints

- Wails **v3 beta** (`v3.0.0-beta.26`, pré-release); no Linux usa **gtk4 + webkitgtk-6.0** (`wails3 doctor` confirma).
- Módulo Go: `sqliteviewer`. Driver SQLite: `modernc.org/sqlite` (Go puro).
- Abrir sempre com `PRAGMA foreign_keys=ON`. Nunca criar arquivo de banco ao abrir um caminho inexistente.
- SQL montado só no backend; identificadores validados contra o schema e citados com `"..."` (aspas duplicadas escapadas); valores só como parâmetros.
- Linha identificada pela PK; sem PK usa `rowid`; views e tabelas sem chave são somente leitura.
- BLOB aparece como `<blob N bytes>` e não é editável. NULL é distinto de string vazia.
- Um banco aberto por vez; recentes em JSON no diretório de config do usuário; reabrir o último ao iniciar; aceitar caminho por linha de comando.
- Fora do v1: editor SQL, diagrama ER, edição inline, múltiplos bancos, edição de BLOB.
- Textos de UI e mensagens de erro ao usuário em **pt-BR**; identificadores e comentários de código em inglês.

## Review Focus

- **Arquivo inválido ao abrir:** caminho inexistente não pode criar arquivo; arquivo que não é SQLite dá erro claro. → Task 2.
- **Identificadores estranhos:** tabela `we"ird` e coluna `select`/`my col` (aspas, espaços, palavra reservada). → Tasks 3, 4, 5.
- **FK sem coluna alvo** (`REFERENCES users`), FK composta e auto-referência. → Task 3.
- **Fidelidade de valores:** inteiro > 2^53, BLOB e coluna `DATETIME` voltam exatamente como armazenados. → Task 4.
- **NULL vs string vazia no formulário**, e campo numérico vazio/ inválido. → Task 12.

---

## Estrutura de arquivos

```
go.mod, main.go                       # main: wiring do Wails (Task 8)
internal/testdb/testdb.go             # helper de teste: SQLite em memória
internal/db/db.go                     # Open(path) → conexão + Info
internal/schema/schema.go             # ListTables, GetTable, FKs
internal/rows/rows.go                 # tipos, normalize, quote, Select
internal/rows/mutate.go               # Insert, Update, Delete, ConstraintError
internal/rows/references.go           # References (registros que apontam para cá)
internal/recents/recents.go           # lista de recentes (JSON)
internal/viewer/service.go            # service Wails
internal/viewer/errors.go             # MarshalError
testdata/seed.sql                     # banco de exemplo para teste manual
frontend/src/lib/{types,api,errors,columnKind,nav,formValues,useRowForm}.ts
frontend/src/stores/viewer.ts
frontend/src/components/{AppSidebar,EmptyState,CellValue,DataGrid,DataTab,StructureTab,RowSheet,FkCombobox,ReferencedBy}.vue
frontend/src/App.vue
```

---

### Task 1: Toolchain e scaffold

**Files:**
- Create: projeto Wails v3 (template `vue`) na raiz; `.gitignore` do template
- Modify: `go.mod`

**Interfaces:**
- Produces: projeto que compila com `wails3 build`; módulo `sqliteviewer`; dependência `modernc.org/sqlite`.

- [ ] **Step 1: Instalar o CLI do Wails v3 (versão fixa)**

Run: `go install -v github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26 && wails3 version`
Expected: imprime `v3.0.0-beta.26`. Se `wails3` não for encontrado, adicionar `~/go/bin` ao PATH (`fish_add_path ~/go/bin`).

- [ ] **Step 2: Instalar a dependência de sistema que falta (pelo usuário)**

O `gtk4` já está instalado; falta `webkitgtk-6.0`. Pedir ao usuário para rodar no prompt:
`! sudo pacman -S --needed webkitgtk-6.0`

- [ ] **Step 3: Verificar o ambiente**

Run: `wails3 doctor`
Expected: dependências de Linux (gtk4, webkitgtk-6.0, gcc) como instaladas. Se algo faltar, o próprio comando indica o pacote.

- [ ] **Step 4: Gerar o projeto na raiz**

Run: `wails3 init -h` e confirmar as flags de nome, template e diretório. Depois:
`wails3 init -n sqliteviewer -t vue -d .`
Se o comando recusar diretório não vazio (existe `docs/`), gerar em `$TMPDIR/sv` e mover tudo, exceto `docs/` e `.git/`, para a raiz.
Run: `grep -rn "changeme" --include=*.go --include=*.mod --include=*.yml . | head` — se o módulo não for `sqliteviewer`, trocar `module` em `go.mod` e os imports para `sqliteviewer`.

- [ ] **Step 5: Adicionar o driver e compilar**

Run: `go get modernc.org/sqlite && go mod tidy && wails3 build`
Expected: build conclui e gera `bin/sqliteviewer`. (O app ainda é o demo do template.)

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "chore: scaffold Wails v3 + Vue project"
```

---

### Task 2: Pacote `db` (abrir banco)

**Files:**
- Create: `internal/testdb/testdb.go`, `internal/db/db.go`
- Test: `internal/db/db_test.go`

**Interfaces:**
- Produces:
  - `testdb.New(t *testing.T, ddl string) *sql.DB`
  - `db.Info{Path, Name string; ReadOnly bool}` (json: `path`, `name`, `readOnly`)
  - `db.Open(path string) (*sql.DB, Info, error)`

- [ ] **Step 1: Criar o helper de teste**

`internal/testdb/testdb.go`:

```go
package testdb

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// New opens an in-memory SQLite database, applies ddl and closes it with the test.
func New(t *testing.T, ddl string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // :memory: is per connection
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	return db
}
```

- [ ] **Step 2: Escrever os testes que falham**

`internal/db/db_test.go`:

```go
package db_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"sqliteviewer/internal/db"
)

func makeDB(t *testing.T, path string) {
	t.Helper()
	c, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_existingDatabase(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ok.db")
	makeDB(t, p)
	conn, info, err := db.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if info.Name != "ok.db" || info.Path != p || info.ReadOnly {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestOpen_missingFileIsNotCreated(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.db")
	if _, _, err := db.Open(p); err == nil {
		t.Fatal("expected error for missing file")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file must not be created, stat err = %v", err)
	}
}

func TestOpen_notADatabase(t *testing.T) {
	p := filepath.Join(t.TempDir(), "text.db")
	content := make([]byte, 0, 300)
	for len(content) < 300 {
		content = append(content, []byte("this is plain text, not sqlite. ")...)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Open(p); err == nil {
		t.Fatal("expected error for non-sqlite file")
	}
}

func TestOpen_directoryIsRejected(t *testing.T) {
	if _, _, err := db.Open(t.TempDir()); err == nil {
		t.Fatal("expected error for directory")
	}
}

func TestOpen_enablesForeignKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fk.db")
	makeDB(t, p)
	conn, _, err := db.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var on int
	if err := conn.QueryRow(`PRAGMA foreign_keys`).Scan(&on); err != nil || on != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", on, err)
	}
}

func TestOpen_readOnlyFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	p := filepath.Join(t.TempDir(), "ro.db")
	makeDB(t, p)
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	conn, info, err := db.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if !info.ReadOnly {
		t.Fatal("expected ReadOnly = true")
	}
	if _, err := conn.Exec(`INSERT INTO t (v) VALUES ('x')`); err == nil {
		t.Fatal("write must fail on read-only database")
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `go test ./internal/db/ -v`
Expected: FAIL (`db.Open` não definido; pacote não compila).

- [ ] **Step 4: Implementar**

`internal/db/db.go`:

```go
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
```

- [ ] **Step 5: Rodar e ver passar**

Run: `go test ./internal/db/ -v`
Expected: PASS em todos (o de read-only é pulado se rodar como root).

- [ ] **Step 6: Commit**

```bash
git add internal/testdb internal/db && git commit -m "feat(db): open SQLite files safely"
```

---

### Task 3: Pacote `schema`

**Files:**
- Create: `internal/schema/schema.go`
- Test: `internal/schema/schema_test.go`

**Interfaces:**
- Consumes: `testdb.New`
- Produces (json em camelCase):
  - `schema.TableInfo{Name, Kind string}` (`kind`: `"table"` | `"view"`)
  - `schema.Column{Name, Type string; NotNull bool; Default *string; PK int; Generated bool}`
  - `schema.ForeignKey{Table string; From, To []string; OnUpdate, OnDelete string}` (`To[i]` pode ser `""` se a tabela referenciada não existir)
  - `schema.IncomingFK{Table string; From, To []string}` (`Table` é a tabela de origem; `From` são colunas dela; `To` são colunas desta tabela)
  - `schema.TableSchema{Name, Kind string; ReadOnly bool; Columns []Column; PrimaryKey []string; UsesRowID bool; KeyColumns []string; ForeignKeys []ForeignKey; Incoming []IncomingFK}`
  - `schema.ListTables(db *sql.DB) ([]TableInfo, error)`
  - `schema.GetTable(db *sql.DB, name string) (*TableSchema, error)`
  - Regra: `KeyColumns` = PK, ou `["rowid"]` (com `UsesRowID=true`) se não há PK e a tabela tem rowid; senão vazio. `ReadOnly` = view ou `len(KeyColumns)==0`.

- [ ] **Step 1: Escrever os testes que falham**

`internal/schema/schema_test.go`:

```go
package schema_test

import (
	"reflect"
	"testing"

	"sqliteviewer/internal/schema"
	"sqliteviewer/internal/testdb"
)

const fixture = `
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE, manager_id INTEGER REFERENCES users(id));
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT DEFAULT 'untitled');
CREATE TABLE tags (a INTEGER, b INTEGER, label TEXT, PRIMARY KEY (a, b)) WITHOUT ROWID;
CREATE TABLE tag_notes (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER, FOREIGN KEY (a, b) REFERENCES tags(a, b));
CREATE TABLE log (msg TEXT);
CREATE TABLE implicit_child (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users);
CREATE TABLE "we""ird name" ("select" TEXT, "my col" INTEGER, PRIMARY KEY ("select"));
CREATE TABLE gen (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) VIRTUAL);
CREATE TABLE auto (id INTEGER PRIMARY KEY AUTOINCREMENT);
CREATE VIEW v_posts AS SELECT id, title FROM posts;
`

func TestListTables(t *testing.T) {
	db := testdb.New(t, fixture)
	got, err := schema.ListTables(db)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, ti := range got {
		kinds[ti.Name] = ti.Kind
	}
	if kinds["users"] != "table" || kinds["v_posts"] != "view" || kinds[`we"ird name`] != "table" {
		t.Fatalf("unexpected kinds: %v", kinds)
	}
	if _, ok := kinds["sqlite_sequence"]; ok {
		t.Fatal("internal sqlite_ tables must be hidden")
	}
	if got[len(got)-1].Name != "v_posts" {
		t.Fatalf("views must come after tables, got last = %s", got[len(got)-1].Name)
	}
}

func TestGetTable_columnsAndKeys(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "posts")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.PrimaryKey, []string{"id"}) || !reflect.DeepEqual(s.KeyColumns, []string{"id"}) || s.UsesRowID || s.ReadOnly {
		t.Fatalf("keys wrong: %+v", s)
	}
	var title schema.Column
	for _, c := range s.Columns {
		if c.Name == "title" {
			title = c
		}
	}
	if title.Default == nil || *title.Default != "'untitled'" {
		t.Fatalf("default wrong: %+v", title)
	}
}

func TestGetTable_noPKUsesRowID(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "log")
	if err != nil {
		t.Fatal(err)
	}
	if !s.UsesRowID || !reflect.DeepEqual(s.KeyColumns, []string{"rowid"}) || s.ReadOnly {
		t.Fatalf("rowid handling wrong: %+v", s)
	}
}

func TestGetTable_withoutRowIDCompositePK(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "tags")
	if err != nil {
		t.Fatal(err)
	}
	if s.UsesRowID || !reflect.DeepEqual(s.KeyColumns, []string{"a", "b"}) {
		t.Fatalf("composite key wrong: %+v", s)
	}
}

func TestGetTable_viewIsReadOnly(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "v_posts")
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != "view" || !s.ReadOnly {
		t.Fatalf("view must be read-only: %+v", s)
	}
}

func TestGetTable_weirdIdentifiers(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, `we"ird name`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.PrimaryKey, []string{"select"}) || len(s.Columns) != 2 || s.Columns[1].Name != "my col" {
		t.Fatalf("weird identifiers wrong: %+v", s)
	}
}

func TestGetTable_generatedColumn(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "gen")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Columns) != 2 || s.Columns[0].Generated || !s.Columns[1].Generated {
		t.Fatalf("generated flag wrong: %+v", s.Columns)
	}
}

func TestGetTable_foreignKeysOutAndIn(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "users")
	if err != nil {
		t.Fatal(err)
	}
	// self reference goes out and comes in
	if len(s.ForeignKeys) != 1 || s.ForeignKeys[0].Table != "users" {
		t.Fatalf("outgoing wrong: %+v", s.ForeignKeys)
	}
	in := map[string]schema.IncomingFK{}
	for _, f := range s.Incoming {
		in[f.Table] = f
	}
	if !reflect.DeepEqual(in["posts"].From, []string{"user_id"}) || !reflect.DeepEqual(in["posts"].To, []string{"id"}) {
		t.Fatalf("incoming from posts wrong: %+v", in["posts"])
	}
	if _, ok := in["users"]; !ok {
		t.Fatal("self reference must appear as incoming")
	}
}

func TestGetTable_implicitFKTargetResolvesToPK(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "implicit_child")
	if err != nil {
		t.Fatal(err)
	}
	fk := s.ForeignKeys[0]
	if fk.Table != "users" || !reflect.DeepEqual(fk.To, []string{"id"}) {
		t.Fatalf("implicit target not resolved: %+v", fk)
	}
	users, _ := schema.GetTable(db, "users")
	found := false
	for _, f := range users.Incoming {
		if f.Table == "implicit_child" && reflect.DeepEqual(f.To, []string{"id"}) {
			found = true
		}
	}
	if !found {
		t.Fatal("implicit FK must show up as incoming on the parent")
	}
}

func TestGetTable_compositeFK(t *testing.T) {
	db := testdb.New(t, fixture)
	s, err := schema.GetTable(db, "tag_notes")
	if err != nil {
		t.Fatal(err)
	}
	fk := s.ForeignKeys[0]
	if !reflect.DeepEqual(fk.From, []string{"a", "b"}) || !reflect.DeepEqual(fk.To, []string{"a", "b"}) {
		t.Fatalf("composite FK wrong: %+v", fk)
	}
}

func TestGetTable_unknown(t *testing.T) {
	db := testdb.New(t, fixture)
	if _, err := schema.GetTable(db, "nope"); err == nil {
		t.Fatal("expected error for unknown table")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/schema/ -v`
Expected: FAIL (pacote `schema` sem implementação).

- [ ] **Step 3: Implementar**

`internal/schema/schema.go`:

```go
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
	var out []TableInfo
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
	s := &TableSchema{Name: name, Kind: kind, Columns: cols}

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
	if s.Incoming, err = incoming(db, name); err != nil {
		return nil, err
	}
	return s, nil
}

func columns(db *sql.DB, table string) ([]Column, error) {
	rs, err := db.Query(`SELECT name, type, "notnull", dflt_value, pk, hidden FROM pragma_table_xinfo(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Column
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
	var out []string
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
	var fks []ForeignKey
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
			fks = append(fks, ForeignKey{Table: ref, OnUpdate: onU, OnDelete: onD})
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
				return nil, err
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
	var out []IncomingFK
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
				out = append(out, IncomingFK{Table: ti.Name, From: fk.From, To: fk.To})
			}
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/schema/ -v`
Expected: PASS em todos. Se `pragma_table_list` não existir, a versão do SQLite embutida é antiga; rodar `go get -u modernc.org/sqlite`.

- [ ] **Step 5: Commit**

```bash
git add internal/schema && git commit -m "feat(schema): read tables, columns and foreign keys"
```

---

### Task 4: `rows` — leitura (Select)

**Files:**
- Create: `internal/rows/rows.go`
- Test: `internal/rows/select_test.go`

**Interfaces:**
- Consumes: `schema.TableSchema`, `schema.Column`
- Produces (json camelCase):
  - `rows.Cond{Column string; Value any}`
  - `rows.Query{Table string; Page, PageSize int; OrderBy string; Desc bool; Filter string; Where []Cond}`
  - `rows.Row{Key, Values map[string]any}`
  - `rows.Page{Rows []Row; Total int64; Page, PageSize int}`
  - `rows.Select(db *sql.DB, s *schema.TableSchema, q Query) (*Page, error)`
  - helpers de pacote: `quote(id string) string`, `normalize(v any) any`, `whereKey(...)` (Task 5)
  - Valores: NULL → `nil`; BLOB → `map[string]any{"$blob": N}`; inteiro com |n| > 2^53 → string decimal; colunas declaradas como DATE/TIME são lidas via `CAST(... AS TEXT)` (texto exatamente como armazenado).

- [ ] **Step 1: Escrever os testes que falham**

`internal/rows/select_test.go`:

```go
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
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/rows/ -v`
Expected: FAIL (pacote `rows` sem implementação).

- [ ] **Step 3: Implementar**

`internal/rows/rows.go`:

```go
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

	order := ""
	if q.OrderBy != "" {
		if !hasColumn(s, q.OrderBy) {
			return nil, fmt.Errorf("coluna desconhecida: %q", q.OrderBy)
		}
		order = " ORDER BY " + quote(q.OrderBy)
		if q.Desc {
			order += " DESC"
		}
		// tie-breaker keeps pagination stable
		for _, k := range s.KeyColumns {
			if s.UsesRowID {
				order += ", rowid"
			} else {
				order += ", " + quote(k)
			}
		}
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
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/rows/ -v`
Expected: PASS em todos.

- [ ] **Step 5: Commit**

```bash
git add internal/rows && git commit -m "feat(rows): paginated, ordered, filtered reads"
```

---

### Task 5: `rows` — escrita (Insert, Update, Delete)

**Files:**
- Create: `internal/rows/mutate.go`
- Test: `internal/rows/mutate_test.go`

**Interfaces:**
- Consumes: `quote`, `normalize`, `hasColumn` (Task 4), `schema.TableSchema`
- Produces:
  - `rows.ErrReadOnly`
  - `rows.ConstraintError{Kind string; Columns []string; Msg string}` (`Error()` retorna `Msg`; Kind ∈ `NOT NULL`, `UNIQUE`, `PRIMARY KEY`, `CHECK`, `FOREIGN KEY`)
  - `rows.Insert(db, s, values map[string]any) (map[string]any, error)` — retorna a chave da nova linha
  - `rows.Update(db, s, key, values map[string]any) error`
  - `rows.Delete(db, s, key map[string]any) error`
  - `rows.whereKey(s, key) (string, []any, error)` (interno, usado também em Task 6)

- [ ] **Step 1: Escrever os testes que falham**

`internal/rows/mutate_test.go`:

```go
package rows_test

import (
	"errors"
	"reflect"
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
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/rows/ -run 'Insert|Update|Delete|Mutations' -v`
Expected: FAIL (`rows.Insert`, `ConstraintError` etc. indefinidos).

- [ ] **Step 3: Implementar**

`internal/rows/mutate.go`:

```go
package rows

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"sqliteviewer/internal/schema"
)

var ErrReadOnly = errors.New("esta tabela é somente leitura")

// ConstraintError is a violated constraint, with the columns involved when SQLite reports them.
type ConstraintError struct {
	Kind    string   `json:"kind"`
	Columns []string `json:"columns"`
	Msg     string   `json:"message"`
}

func (e *ConstraintError) Error() string { return e.Msg }

var constraintRe = regexp.MustCompile(`(NOT NULL|UNIQUE|PRIMARY KEY|CHECK|FOREIGN KEY) constraint failed(?:: ([^()]+))?`)

func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	m := constraintRe.FindStringSubmatch(err.Error())
	if m == nil {
		return err
	}
	ce := &ConstraintError{Kind: m[1]}
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
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/rows/ -v`
Expected: PASS em todos (incluindo os da Task 4). Se o texto do erro do `modernc` divergir do regex, ajustar `constraintRe` olhando a mensagem real impressa pelo teste que falhar.

- [ ] **Step 5: Commit**

```bash
git add internal/rows && git commit -m "feat(rows): insert, update, delete with constraint errors"
```

---

### Task 6: `rows` — "Referenciado por"

**Files:**
- Create: `internal/rows/references.go`
- Test: `internal/rows/references_test.go`

**Interfaces:**
- Consumes: `whereKey`, `quote`, `schema.TableSchema.Incoming`
- Produces: `rows.RefCount{Table string; From, To []string; Count int64}` (json: `table`, `from`, `to`, `count`); `rows.References(db, s, key) ([]RefCount, error)` — só entradas com `Count > 0`.

- [ ] **Step 1: Escrever os testes que falham**

`internal/rows/references_test.go`:

```go
package rows_test

import (
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
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/rows/ -run References -v`
Expected: FAIL (`rows.References` indefinido).

- [ ] **Step 3: Implementar**

`internal/rows/references.go`:

```go
package rows

import (
	"database/sql"
	"fmt"
	"strings"

	"sqliteviewer/internal/schema"
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
	var out []RefCount
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
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/rows/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rows && git commit -m "feat(rows): count rows referencing a record"
```

---

### Task 7: Pacote `recents`

**Files:**
- Create: `internal/recents/recents.go`
- Test: `internal/recents/recents_test.go`

**Interfaces:**
- Produces:
  - `recents.New(path string) *Store`
  - `recents.DefaultPath() (string, error)` — `<UserConfigDir>/sqliteviewer/recents.json`
  - `(*Store).List() []string` — nunca falha; arquivo ausente/corrompido → vazio
  - `(*Store).Add(p string) error` — move para o topo, remove duplicata, máx. 10
  - `(*Store).Remove(p string) error`
  - `(*Store).Last() string` — primeiro item cujo arquivo ainda existe, ou `""`

- [ ] **Step 1: Escrever os testes que falham**

`internal/recents/recents_test.go`:

```go
package recents_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sqliteviewer/internal/recents"
)

func newStore(t *testing.T) (*recents.Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg", "recents.json") // parent dir does not exist yet
	return recents.New(p), p
}

func TestList_missingFileIsEmpty(t *testing.T) {
	s, _ := newStore(t)
	if got := s.List(); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestList_corruptFileIsEmpty(t *testing.T) {
	s, p := newStore(t)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("{not json"), 0o644)
	if got := s.List(); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	if err := s.Add("/a.db"); err != nil { // and it recovers on next write
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.List(), []string{"/a.db"}) {
		t.Fatalf("did not recover: %v", s.List())
	}
}

func TestAdd_mruDedupeAndCap(t *testing.T) {
	s, _ := newStore(t)
	s.Add("/a.db")
	s.Add("/b.db")
	s.Add("/a.db")
	if got := s.List(); !reflect.DeepEqual(got, []string{"/a.db", "/b.db"}) {
		t.Fatalf("mru wrong: %v", got)
	}
	for i := 0; i < 15; i++ {
		s.Add(fmt.Sprintf("/f%d.db", i))
	}
	if got := s.List(); len(got) != 10 || got[0] != "/f14.db" {
		t.Fatalf("cap wrong: %v", got)
	}
}

func TestRemove(t *testing.T) {
	s, _ := newStore(t)
	s.Add("/a.db")
	s.Add("/b.db")
	if err := s.Remove("/a.db"); err != nil {
		t.Fatal(err)
	}
	if got := s.List(); !reflect.DeepEqual(got, []string{"/b.db"}) {
		t.Fatalf("remove wrong: %v", got)
	}
}

func TestLast_skipsMissingFiles(t *testing.T) {
	s, _ := newStore(t)
	real := filepath.Join(t.TempDir(), "real.db")
	os.WriteFile(real, []byte("x"), 0o644)
	s.Add(real)
	s.Add("/definitely/missing.db")
	if got := s.Last(); got != real {
		t.Fatalf("Last = %q, want %q", got, real)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/recents/ -v`
Expected: FAIL (pacote sem implementação).

- [ ] **Step 3: Implementar**

`internal/recents/recents.go`:

```go
package recents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const maxEntries = 10

type Store struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Store { return &Store{path: path} }

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sqliteviewer", "recents.json"), nil
}

func (s *Store) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) load() []string {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal(b, &list); err != nil {
		return []string{}
	}
	return list
}

func (s *Store) Add(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []string{p}
	for _, x := range s.load() {
		if x != p {
			list = append(list, x)
		}
	}
	if len(list) > maxEntries {
		list = list[:maxEntries]
	}
	return s.save(list)
}

func (s *Store) Remove(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []string{}
	for _, x := range s.load() {
		if x != p {
			list = append(list, x)
		}
	}
	return s.save(list)
}

func (s *Store) Last() string {
	for _, p := range s.List() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (s *Store) save(list []string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/recents/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/recents && git commit -m "feat(recents): persist recent databases"
```

---

### Task 8: `viewer.Service`, `MarshalError` e wiring do Wails

**Files:**
- Create: `internal/viewer/service.go`, `internal/viewer/errors.go`, `testdata/seed.sql`
- Modify: `main.go` (substituir o demo), apagar `greetservice.go`
- Test: `internal/viewer/service_test.go`

**Interfaces:**
- Consumes: `db.Open`, `schema.*`, `rows.*`, `recents.Store`
- Produces (métodos exportados = bindings Wails):
  - `viewer.Options{Recents *recents.Store; CLIPath string; Pick func() (string, error)}`
  - `viewer.New(Options) *Service`
  - `(*Service).InitialDB() (*db.Info, error)` — abre `CLIPath` se houver (erro propagado); senão reabre o último recente (falha silenciosa → `nil, nil`)
  - `OpenDialog() (*db.Info, error)` — `nil, nil` se cancelado
  - `OpenPath(path string) (*db.Info, error)`
  - `CurrentDB() *db.Info`, `Close() error`
  - `Recents() []string`, `ForgetRecent(path string) error`
  - `ListTables() ([]schema.TableInfo, error)`, `GetTable(name string) (*schema.TableSchema, error)`
  - `QueryRows(q rows.Query) (*rows.Page, error)`
  - `InsertRow(table string, values map[string]any) (map[string]any, error)`
  - `UpdateRow(table string, key, values map[string]any) error`
  - `DeleteRow(table string, key map[string]any) error`
  - `References(table string, key map[string]any) ([]rows.RefCount, error)`
  - `viewer.MarshalError(err error) []byte` — constraint → JSON `{"type":"constraint","kind":…,"columns":[…],"message":…}`; outros → `nil`
  - Sem banco aberto, métodos de dados retornam `viewer.ErrNoDatabase` ("nenhum banco aberto").

- [ ] **Step 1: Escrever os testes que falham**

`internal/viewer/service_test.go`:

```go
package viewer_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"sqliteviewer/internal/recents"
	"sqliteviewer/internal/rows"
	"sqliteviewer/internal/viewer"
)

func makeDB(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "app.db")
	c, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, err = c.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE);
		CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id));
		INSERT INTO users (email) VALUES ('a@x.com');`)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newService(t *testing.T, cli string) (*viewer.Service, *recents.Store) {
	t.Helper()
	rc := recents.New(filepath.Join(t.TempDir(), "recents.json"))
	return viewer.New(viewer.Options{Recents: rc, CLIPath: cli}), rc
}

func TestNoDatabaseOpen(t *testing.T) {
	s, _ := newService(t, "")
	if _, err := s.ListTables(); !errors.Is(err, viewer.ErrNoDatabase) {
		t.Fatalf("expected ErrNoDatabase, got %v", err)
	}
	if s.CurrentDB() != nil {
		t.Fatal("CurrentDB must be nil")
	}
}

func TestOpenPath_recordsRecentAndServesData(t *testing.T) {
	s, rc := newService(t, "")
	p := makeDB(t)
	info, err := s.OpenPath(p)
	if err != nil || info.Name != "app.db" {
		t.Fatalf("OpenPath: %+v %v", info, err)
	}
	if got := rc.List(); len(got) != 1 || got[0] != p {
		t.Fatalf("recents wrong: %v", got)
	}
	tables, err := s.ListTables()
	if err != nil || len(tables) != 2 {
		t.Fatalf("ListTables: %+v %v", tables, err)
	}
	page, err := s.QueryRows(rows.Query{Table: "users"})
	if err != nil || page.Total != 1 {
		t.Fatalf("QueryRows: %+v %v", page, err)
	}
}

func TestOpenPath_failureKeepsPreviousDatabase(t *testing.T) {
	s, _ := newService(t, "")
	if _, err := s.OpenPath(makeDB(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenPath("/definitely/missing.db"); err == nil {
		t.Fatal("expected error")
	}
	if s.CurrentDB() == nil {
		t.Fatal("previous database must stay open after a failed open")
	}
}

func TestCRUDRoundTrip(t *testing.T) {
	s, _ := newService(t, "")
	s.OpenPath(makeDB(t))
	key, err := s.InsertRow("users", map[string]any{"email": "b@x.com"})
	if err != nil || key["id"] != int64(2) {
		t.Fatalf("InsertRow: %v %v", key, err)
	}
	if err := s.UpdateRow("users", key, map[string]any{"email": "bb@x.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertRow("posts", map[string]any{"user_id": float64(2)}); err != nil {
		t.Fatal(err)
	}
	refs, err := s.References("users", key)
	if err != nil || len(refs) != 1 || refs[0].Count != 1 {
		t.Fatalf("References: %+v %v", refs, err)
	}
	if err := s.DeleteRow("users", key); err == nil {
		t.Fatal("delete must be blocked by FK")
	}
}

func TestInitialDB(t *testing.T) {
	p := makeDB(t)

	s, _ := newService(t, p) // CLI path wins
	if info, err := s.InitialDB(); err != nil || info == nil {
		t.Fatalf("CLI open: %+v %v", info, err)
	}

	s, _ = newService(t, "/missing.db") // CLI error is reported
	if _, err := s.InitialDB(); err == nil {
		t.Fatal("bad CLI path must return an error")
	}

	s, rc := newService(t, "") // last recent is reopened silently
	rc.Add(p)
	if info, err := s.InitialDB(); err != nil || info == nil {
		t.Fatalf("recent reopen: %+v %v", info, err)
	}

	s, rc = newService(t, "")
	rc.Add("/gone.db") // missing recent: no error, no db
	if info, err := s.InitialDB(); err != nil || info != nil {
		t.Fatalf("missing recent: %+v %v", info, err)
	}
}

func TestMarshalError(t *testing.T) {
	ce := &rows.ConstraintError{Kind: "NOT NULL", Columns: []string{"email"}, Msg: "email é obrigatório"}
	b := viewer.MarshalError(ce)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("not JSON: %s", b)
	}
	if got["type"] != "constraint" || got["kind"] != "NOT NULL" || got["message"] != "email é obrigatório" {
		t.Fatalf("payload wrong: %v", got)
	}
	if viewer.MarshalError(errors.New("boom")) != nil {
		t.Fatal("non-constraint errors must fall back to default handling")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/viewer/ -v`
Expected: FAIL (pacote `viewer` sem implementação).

- [ ] **Step 3: Implementar o service**

`internal/viewer/service.go`:

```go
package viewer

import (
	"database/sql"
	"errors"
	"sync"

	appdb "sqliteviewer/internal/db"
	"sqliteviewer/internal/recents"
	"sqliteviewer/internal/rows"
	"sqliteviewer/internal/schema"
)

var ErrNoDatabase = errors.New("nenhum banco aberto")

type Options struct {
	Recents *recents.Store
	CLIPath string
	Pick    func() (string, error) // native file picker; "" when cancelled
}

// Service is the single Wails service. Every exported method is callable from the frontend.
type Service struct {
	mu   sync.RWMutex
	opts Options
	conn *sql.DB
	info *appdb.Info
}

func New(o Options) *Service { return &Service{opts: o} }

func (s *Service) current() (*sql.DB, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.conn == nil {
		return nil, ErrNoDatabase
	}
	return s.conn, nil
}

func (s *Service) table(name string) (*sql.DB, *schema.TableSchema, error) {
	conn, err := s.current()
	if err != nil {
		return nil, nil, err
	}
	ts, err := schema.GetTable(conn, name)
	if err != nil {
		return nil, nil, err
	}
	return conn, ts, nil
}

func (s *Service) InitialDB() (*appdb.Info, error) {
	if s.opts.CLIPath != "" {
		return s.OpenPath(s.opts.CLIPath)
	}
	if last := s.opts.Recents.Last(); last != "" {
		info, err := s.OpenPath(last)
		if err != nil {
			return nil, nil // auto-reopen failures are not worth a dialog
		}
		return info, nil
	}
	return nil, nil
}

func (s *Service) OpenDialog() (*appdb.Info, error) {
	if s.opts.Pick == nil {
		return nil, errors.New("seletor de arquivos indisponível")
	}
	path, err := s.opts.Pick()
	if err != nil || path == "" {
		return nil, err
	}
	return s.OpenPath(path)
}

func (s *Service) OpenPath(path string) (*appdb.Info, error) {
	conn, info, err := appdb.Open(path)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	old := s.conn
	s.conn, s.info = conn, &info
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
	_ = s.opts.Recents.Add(info.Path)
	return &info, nil
}

func (s *Service) CurrentDB() *appdb.Info {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn, s.info = nil, nil
	return err
}

func (s *Service) Recents() []string { return s.opts.Recents.List() }

func (s *Service) ForgetRecent(path string) error { return s.opts.Recents.Remove(path) }

func (s *Service) ListTables() ([]schema.TableInfo, error) {
	conn, err := s.current()
	if err != nil {
		return nil, err
	}
	return schema.ListTables(conn)
}

func (s *Service) GetTable(name string) (*schema.TableSchema, error) {
	_, ts, err := s.table(name)
	return ts, err
}

func (s *Service) QueryRows(q rows.Query) (*rows.Page, error) {
	conn, ts, err := s.table(q.Table)
	if err != nil {
		return nil, err
	}
	return rows.Select(conn, ts, q)
}

func (s *Service) InsertRow(table string, values map[string]any) (map[string]any, error) {
	conn, ts, err := s.table(table)
	if err != nil {
		return nil, err
	}
	return rows.Insert(conn, ts, values)
}

func (s *Service) UpdateRow(table string, key, values map[string]any) error {
	conn, ts, err := s.table(table)
	if err != nil {
		return err
	}
	return rows.Update(conn, ts, key, values)
}

func (s *Service) DeleteRow(table string, key map[string]any) error {
	conn, ts, err := s.table(table)
	if err != nil {
		return err
	}
	return rows.Delete(conn, ts, key)
}

func (s *Service) References(table string, key map[string]any) ([]rows.RefCount, error) {
	conn, ts, err := s.table(table)
	if err != nil {
		return nil, err
	}
	return rows.References(conn, ts, key)
}
```

`internal/viewer/errors.go`:

```go
package viewer

import (
	"encoding/json"
	"errors"

	"sqliteviewer/internal/rows"
)

// MarshalError makes constraint violations reach the frontend as structured data
// (available as `error.cause` in JS). Other errors use Wails' default handling.
func MarshalError(err error) []byte {
	var ce *rows.ConstraintError
	if !errors.As(err, &ce) {
		return nil
	}
	b, jerr := json.Marshal(map[string]any{
		"type":    "constraint",
		"kind":    ce.Kind,
		"columns": ce.Columns,
		"message": ce.Msg,
	})
	if jerr != nil {
		return nil
	}
	return b
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/... -v`
Expected: PASS em todos os pacotes.

- [ ] **Step 5: Substituir o demo em `main.go`**

Apagar `greetservice.go`. Substituir o conteúdo de `main.go` mantendo o `//go:embed` do template:

```go
package main

import (
	"embed"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"sqliteviewer/internal/recents"
	"sqliteviewer/internal/viewer"
)

//go:embed all:frontend/dist
var assets embed.FS

// cliPath returns the first argument that is not a flag, e.g. `sqliteviewer my.db`.
func cliPath(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

func main() {
	var app *application.App

	cfgPath, err := recents.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	svc := viewer.New(viewer.Options{
		Recents: recents.New(cfgPath),
		CLIPath: cliPath(os.Args[1:]),
		Pick: func() (string, error) {
			return app.Dialog.OpenFile().
				SetTitle("Abrir banco SQLite").
				AddFilter("Bancos SQLite", "*.db;*.sqlite;*.sqlite3;*.db3").
				AddFilter("Todos os arquivos", "*.*").
				PromptForSingleSelection()
		},
	})

	app = application.New(application.Options{
		Name:         "SQLite Viewer",
		Description:  "Visualizador simples de bancos SQLite",
		Services:     []application.Service{application.NewService(svc)},
		MarshalError: viewer.MarshalError,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "SQLite Viewer",
		Width:  1280,
		Height: 800,
		URL:    "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
```

Remover também do frontend o uso de `GreetService` (será substituído na Task 9: apagar `frontend/src/components/HelloWorld.vue` e deixar `App.vue` mínimo) para o `vue-tsc` não quebrar com bindings que deixam de existir.

- [ ] **Step 6: Gerar bindings e conferir o layout gerado**

Run: `wails3 generate bindings -ts` (confirmar flags com `wails3 generate bindings -h`)
Run: `find frontend/bindings -type f | sort`
Expected: arquivos TS do `viewer` (algo como `frontend/bindings/sqliteviewer/internal/viewer/service.ts` + `index.ts`) e modelos de `schema`, `rows`, `db`. **Anotar o caminho de import real** — a Task 9 usa `lib/api.ts` como único ponto que importa os bindings.

- [ ] **Step 7: Banco de exemplo para teste manual**

`testdata/seed.sql`:

```sql
PRAGMA foreign_keys = ON;
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE, name TEXT, active INTEGER DEFAULT 1, created DATETIME DEFAULT CURRENT_TIMESTAMP, manager_id INTEGER REFERENCES users(id));
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT DEFAULT 'untitled', body TEXT);
CREATE TABLE tags (a INTEGER, b INTEGER, label TEXT, PRIMARY KEY (a, b)) WITHOUT ROWID;
CREATE TABLE tag_notes (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER, FOREIGN KEY (a, b) REFERENCES tags(a, b));
CREATE TABLE log (msg TEXT);
CREATE TABLE "we""ird name" ("select" TEXT PRIMARY KEY, "my col" INTEGER);
CREATE TABLE blobs (id INTEGER PRIMARY KEY, data BLOB, big INTEGER);
CREATE VIEW v_posts AS SELECT p.id, p.title, u.email FROM posts p JOIN users u ON u.id = p.user_id;
INSERT INTO users (email, name, manager_id) VALUES ('ana@x.com','Ana',NULL), ('bruno@x.com','Bruno',1), ('carla@x.com','Carla',1);
INSERT INTO posts (user_id, title, body) VALUES (1,'Primeiro','texto longo...'), (1,'Segundo',NULL), (2,'Do Bruno',''), (3,'Da Carla','x');
INSERT INTO tags VALUES (1,1,'a'), (1,2,'b');
INSERT INTO tag_notes (a,b) VALUES (1,1);
INSERT INTO log VALUES ('hello'), ('world');
INSERT INTO "we""ird name" VALUES ('k1', 10), ('k2', NULL);
INSERT INTO blobs (data, big) VALUES (x'DEADBEEF', 9007199254740993);
```

Run: `python3 -c "import sqlite3; sqlite3.connect('testdata/sample.db').executescript(open('testdata/seed.sql').read())"`
Expected: cria `testdata/sample.db`. Adicionar `testdata/sample.db` ao `.gitignore`.

- [ ] **Step 8: Commit**

```bash
git add -A && git commit -m "feat(viewer): Wails service, structured errors and app wiring"
```

---

### Task 9: Fundação do frontend

**Files:**
- Modify: `frontend/package.json`, `frontend/tsconfig.json`, `frontend/vite.config.ts`, `frontend/index.html`, `frontend/src/main.ts`, `frontend/src/App.vue`
- Create: `frontend/vitest.config.ts`, `frontend/src/style.css`, `frontend/src/lib/types.ts`, `frontend/src/lib/api.ts`, `frontend/src/lib/errors.ts`, `frontend/src/lib/columnKind.ts`, componentes `ui/` do shadcn-vue
- Delete: `frontend/src/components/HelloWorld.vue`, `frontend/public/style.css` (e o `<link>` em `index.html`)
- Test: `frontend/src/lib/errors.test.ts`, `frontend/src/lib/columnKind.test.ts`

**Interfaces:**
- Consumes: bindings gerados (Task 8, Step 6)
- Produces:
  - tipos em `lib/types.ts` (espelham o JSON do Go): `Column, ForeignKey, IncomingFK, TableInfo, TableSchema, Row, Page, Cond, RowQuery, RefCount, DbInfo, Values`, `isBlob(v)`
  - `lib/api.ts`: `api.initialDb(): Promise<DbInfo|null>`, `openDialog()`, `openPath(path)`, `recents(): Promise<string[]>`, `forgetRecent(path)`, `listTables(): Promise<TableInfo[]>`, `getTable(name): Promise<TableSchema>`, `queryRows(q: RowQuery): Promise<Page>`, `insertRow(table, values): Promise<Values>`, `updateRow(table, key, values): Promise<void>`, `deleteRow(table, key): Promise<void>`, `references(table, key): Promise<RefCount[]>`
  - `lib/errors.ts`: `AppError{message: string; constraint?: string; columns: string[]}`, `toAppError(e: unknown): AppError`
  - `lib/columnKind.ts`: `InputKind = 'boolean'|'integer'|'number'|'date'|'datetime'|'textarea'|'text'`, `inputKind(decl: string): InputKind`

- [ ] **Step 1: Dependências**

Run (em `frontend/`):
```
npm i -D typescript@latest vue-tsc@latest tailwindcss @tailwindcss/vite @types/node vitest @vue/test-utils happy-dom
npm i pinia @tanstack/vue-table vue-sonner
```
O template traz TypeScript 4.9 e vue-tsc 1.x, antigos demais para o shadcn-vue.

- [ ] **Step 2: Tailwind, alias `@` e Vitest**

`frontend/vite.config.ts`:

```ts
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";
import { fileURLToPath } from "node:url";

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  plugins: [vue(), tailwindcss(), wails("./bindings")],
});
```

`frontend/vitest.config.ts`:

```ts
import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath } from "node:url";

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: { environment: "happy-dom" },
});
```

Em `frontend/tsconfig.json`, em `compilerOptions`, adicionar `"baseUrl": "."` e `"paths": { "@/*": ["./src/*"] }`. Em `package.json`, adicionar o script `"test": "vitest run"`.

`frontend/src/style.css`: `@import "tailwindcss";` (o `shadcn-vue init` acrescenta os tokens de tema).
Em `frontend/src/main.ts`:

```ts
import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import "./style.css";

createApp(App).use(createPinia()).mount("#app");
```

Apagar `src/components/HelloWorld.vue` e `public/style.css`, remover o `<link>` para `style.css` do `index.html`, e deixar `App.vue` como `<template><div>SQLite Viewer</div></template>`.

- [ ] **Step 3: shadcn-vue**

Run (em `frontend/`): `npx shadcn-vue@latest init` (aceitar os padrões; cor base Neutral) e depois
`npx shadcn-vue@latest add button input textarea checkbox label sheet table tabs alert-dialog dropdown-menu sonner badge scroll-area breadcrumb command popover separator`
Run: `npm run build`
Expected: build verde. Se o `vue-tsc` reclamar do `tsconfig`, seguir a mensagem (normalmente `moduleResolution: "bundler"`).

- [ ] **Step 4: Escrever os testes que falham**

`frontend/src/lib/errors.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { toAppError } from "./errors";

describe("toAppError", () => {
  it("reads a structured constraint cause (object)", () => {
    const e = Object.assign(new Error("x"), {
      cause: { type: "constraint", kind: "NOT NULL", columns: ["email"], message: "email é obrigatório" },
    });
    expect(toAppError(e)).toEqual({ message: "email é obrigatório", constraint: "NOT NULL", columns: ["email"] });
  });

  it("reads a structured constraint cause (JSON string)", () => {
    const cause = JSON.stringify({ type: "constraint", kind: "UNIQUE", columns: ["a", "b"], message: "dup" });
    expect(toAppError(Object.assign(new Error("x"), { cause })).columns).toEqual(["a", "b"]);
  });

  it("falls back to the error message", () => {
    expect(toAppError(new Error("boom"))).toEqual({ message: "boom", columns: [] });
    expect(toAppError("texto").message).toBe("texto");
  });

  it("ignores a cause that is not a constraint payload", () => {
    const e = Object.assign(new Error("boom"), { cause: "not json" });
    expect(toAppError(e)).toEqual({ message: "boom", columns: [] });
  });
});
```

`frontend/src/lib/columnKind.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { inputKind } from "./columnKind";

describe("inputKind", () => {
  it.each([
    ["INTEGER", "integer"],
    ["BIGINT", "integer"],
    ["REAL", "number"],
    ["NUMERIC(10,2)", "number"],
    ["BOOLEAN", "boolean"],
    ["DATETIME", "datetime"],
    ["TIMESTAMP", "datetime"],
    ["DATE", "date"],
    ["TEXT", "textarea"],
    ["VARCHAR(80)", "text"],
    ["", "text"],
    ["BLOB", "text"],
  ])("%s -> %s", (decl, kind) => {
    expect(inputKind(decl)).toBe(kind);
  });
});
```

- [ ] **Step 5: Rodar e ver falhar**

Run (em `frontend/`): `npm test`
Expected: FAIL (módulos `./errors` e `./columnKind` não existem).

- [ ] **Step 6: Implementar `types`, `errors`, `columnKind`**

`frontend/src/lib/types.ts`:

```ts
export interface Column { name: string; type: string; notNull: boolean; default: string | null; pk: number; generated: boolean }
export interface ForeignKey { table: string; from: string[]; to: string[]; onUpdate: string; onDelete: string }
export interface IncomingFK { table: string; from: string[]; to: string[] }
export interface TableInfo { name: string; kind: "table" | "view" }
export interface TableSchema {
  name: string; kind: "table" | "view"; readOnly: boolean
  columns: Column[]; primaryKey: string[]; usesRowId: boolean; keyColumns: string[]
  foreignKeys: ForeignKey[]; incoming: IncomingFK[]
}
export type Values = Record<string, unknown>
export interface Row { key: Values; values: Values }
export interface Page { rows: Row[]; total: number; page: number; pageSize: number }
export interface Cond { column: string; value: unknown }
export interface RowQuery { table: string; page: number; pageSize: number; orderBy: string; desc: boolean; filter: string; where: Cond[] }
export interface RefCount { table: string; from: string[]; to: string[]; count: number }
export interface DbInfo { path: string; name: string; readOnly: boolean }

export const isBlob = (v: unknown): v is { $blob: number } =>
  typeof v === "object" && v !== null && "$blob" in v
```

`frontend/src/lib/errors.ts`:

```ts
export interface AppError { message: string; constraint?: string; columns: string[] }

/** Normalizes anything thrown by a binding call; understands the backend's constraint payload in `cause`. */
export function toAppError(e: unknown): AppError {
  let cause: any = (e as any)?.cause
  if (typeof cause === "string") {
    try { cause = JSON.parse(cause) } catch { cause = undefined }
  }
  if (cause && cause.type === "constraint") {
    return { message: cause.message, constraint: cause.kind, columns: cause.columns ?? [] }
  }
  return { message: e instanceof Error ? e.message : String(e), columns: [] }
}
```

`frontend/src/lib/columnKind.ts`:

```ts
export type InputKind = "boolean" | "integer" | "number" | "date" | "datetime" | "textarea" | "text"

export function inputKind(decl: string): InputKind {
  const d = decl.toUpperCase()
  if (d.includes("BOOL")) return "boolean"
  if (d.includes("INT")) return "integer"
  if (/REAL|FLOA|DOUB|NUMERIC|DECIMAL/.test(d)) return "number"
  if (d.includes("DATETIME") || d.includes("TIMESTAMP")) return "datetime"
  if (d.includes("DATE")) return "date"
  if (/TEXT|CLOB/.test(d)) return "textarea"
  return "text"
}
```

- [ ] **Step 7: `lib/api.ts` (único ponto que importa os bindings)**

Usar o caminho de import anotado no Task 8, Step 6. Exemplo (ajustar o caminho se for diferente):

```ts
import { ViewerService } from "../../bindings/sqliteviewer/internal/viewer";
import type { Cond, DbInfo, Page, RefCount, RowQuery, TableInfo, TableSchema, Values } from "./types";

// Generated models are structurally identical to ./types; cast at this boundary only.
const call = <T>(p: Promise<unknown>) => p as Promise<T>;

export const api = {
  initialDb: () => call<DbInfo | null>(ViewerService.InitialDB()),
  openDialog: () => call<DbInfo | null>(ViewerService.OpenDialog()),
  openPath: (path: string) => call<DbInfo>(ViewerService.OpenPath(path)),
  recents: () => call<string[]>(ViewerService.Recents()),
  forgetRecent: (path: string) => call<void>(ViewerService.ForgetRecent(path)),
  listTables: () => call<TableInfo[]>(ViewerService.ListTables()),
  getTable: (name: string) => call<TableSchema>(ViewerService.GetTable(name)),
  queryRows: (q: RowQuery) => call<Page>(ViewerService.QueryRows(q as never)),
  insertRow: (table: string, values: Values) => call<Values>(ViewerService.InsertRow(table, values)),
  updateRow: (table: string, key: Values, values: Values) => call<void>(ViewerService.UpdateRow(table, key, values)),
  deleteRow: (table: string, key: Values) => call<void>(ViewerService.DeleteRow(table, key)),
  references: (table: string, key: Values) => call<RefCount[]>(ViewerService.References(table, key)),
};

export type { Cond };
```

- [ ] **Step 8: Rodar tudo**

Run (em `frontend/`): `npm test && npm run build`
Expected: testes PASS; build verde (confirma também que os imports dos bindings resolvem).

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "feat(frontend): tooling, shadcn-vue, api layer and error helpers"
```

---

### Task 10: Navegação, store e shell da aplicação

**Files:**
- Create: `frontend/src/lib/nav.ts`, `frontend/src/stores/viewer.ts`, `frontend/src/components/AppSidebar.vue`, `frontend/src/components/EmptyState.vue`
- Modify: `frontend/src/App.vue`
- Test: `frontend/src/lib/nav.test.ts`

**Interfaces:**
- Consumes: `api`, tipos (Task 9)
- Produces:
  - `lib/nav.ts`: `NavEntry{table: string; where: Cond[]; label: string}`, `rootEntry(table)`, `pushEntry(trail, e)`, `popTo(trail, index)`, `outgoingTarget(fk: ForeignKey, values: Values): NavEntry`, `incomingTarget(inc: {table; from; to}, values: Values): NavEntry`
  - store `useViewer()` (Pinia): estado `db: DbInfo|null`, `tables: TableInfo[]`, `recents: string[]`, `trail: NavEntry[]`; getter `current: NavEntry|null`; ações `init()`, `openDialog()`, `openPath(p)`, `forget(p)`, `openTable(name)`, `follow(e: NavEntry)`, `backTo(index)`

- [ ] **Step 1: Escrever os testes que falham**

`frontend/src/lib/nav.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { incomingTarget, outgoingTarget, popTo, pushEntry, rootEntry } from "./nav";

describe("trail", () => {
  it("push and popTo", () => {
    let t = rootEntry("posts");
    t = pushEntry(t, { table: "users", where: [{ column: "id", value: 1 }], label: "users" });
    t = pushEntry(t, { table: "posts", where: [], label: "posts" });
    expect(t.map((e) => e.table)).toEqual(["posts", "users", "posts"]);
    expect(popTo(t, 0).map((e) => e.table)).toEqual(["posts"]);
    expect(popTo(t, 1).length).toBe(2);
  });
});

describe("targets", () => {
  it("outgoing FK filters the referenced table by the cell values", () => {
    const e = outgoingTarget({ table: "users", from: ["user_id"], to: ["id"], onUpdate: "", onDelete: "" }, { user_id: 7 });
    expect(e.table).toBe("users");
    expect(e.where).toEqual([{ column: "id", value: 7 }]);
  });

  it("outgoing composite FK keeps all column pairs", () => {
    const e = outgoingTarget({ table: "tags", from: ["a", "b"], to: ["x", "y"], onUpdate: "", onDelete: "" }, { a: 1, b: 2 });
    expect(e.where).toEqual([{ column: "x", value: 1 }, { column: "y", value: 2 }]);
  });

  it("incoming FK filters the referencing table by this row's values", () => {
    const e = incomingTarget({ table: "posts", from: ["user_id"], to: ["id"] }, { id: 3, email: "x" });
    expect(e.table).toBe("posts");
    expect(e.where).toEqual([{ column: "user_id", value: 3 }]);
  });

  it("labels mention the filter", () => {
    const e = incomingTarget({ table: "posts", from: ["user_id"], to: ["id"] }, { id: 3 });
    expect(e.label).toBe("posts (user_id = 3)");
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run (em `frontend/`): `npm test -- nav`
Expected: FAIL (`./nav` não existe).

- [ ] **Step 3: Implementar `nav.ts`**

```ts
import type { Cond, ForeignKey, IncomingFK, Values } from "./types"

export interface NavEntry { table: string; where: Cond[]; label: string }

export const rootEntry = (table: string): NavEntry[] => [{ table, where: [], label: table }]
export const pushEntry = (trail: NavEntry[], e: NavEntry): NavEntry[] => [...trail, e]
export const popTo = (trail: NavEntry[], index: number): NavEntry[] => trail.slice(0, index + 1)

function describe(table: string, where: Cond[]): string {
  const f = where.map((c) => `${c.column} = ${c.value === null ? "NULL" : String(c.value)}`).join(", ")
  return f ? `${table} (${f})` : table
}

function entry(table: string, where: Cond[]): NavEntry {
  return { table, where, label: describe(table, where) }
}

/** Follow an FK from a row: referenced table filtered to the row the FK points at. */
export function outgoingTarget(fk: ForeignKey, values: Values): NavEntry {
  return entry(fk.table, fk.to.map((col, i) => ({ column: col, value: values[fk.from[i]] ?? null })))
}

/** Follow an incoming FK: referencing table filtered to rows pointing at this row. */
export function incomingTarget(inc: Pick<IncomingFK, "table" | "from" | "to">, values: Values): NavEntry {
  return entry(inc.table, inc.from.map((col, i) => ({ column: col, value: values[inc.to[i]] ?? null })))
}
```

- [ ] **Step 4: Rodar e ver passar**

Run (em `frontend/`): `npm test -- nav`
Expected: PASS.

- [ ] **Step 5: Store**

`frontend/src/stores/viewer.ts`:

```ts
import { defineStore } from "pinia"
import { toast } from "vue-sonner"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { popTo, pushEntry, rootEntry, type NavEntry } from "@/lib/nav"
import type { DbInfo, TableInfo } from "@/lib/types"

export const useViewer = defineStore("viewer", {
  state: () => ({
    db: null as DbInfo | null,
    tables: [] as TableInfo[],
    recents: [] as string[],
    trail: [] as NavEntry[],
  }),
  getters: {
    current: (s): NavEntry | null => s.trail[s.trail.length - 1] ?? null,
  },
  actions: {
    async init() {
      this.recents = await api.recents()
      try {
        await this.adopt(await api.initialDb())
      } catch (e) {
        toast.error(toAppError(e).message)
      }
    },
    async adopt(info: DbInfo | null) {
      if (!info) return
      this.db = info
      this.trail = []
      this.tables = await api.listTables()
      this.recents = await api.recents()
    },
    async openDialog() {
      try { await this.adopt(await api.openDialog()) } catch (e) { toast.error(toAppError(e).message) }
    },
    async openPath(path: string) {
      try {
        await this.adopt(await api.openPath(path))
      } catch (e) {
        toast.error(toAppError(e).message)
        await this.forget(path) // dead entry: drop it from the list
      }
    },
    async forget(path: string) {
      await api.forgetRecent(path)
      this.recents = await api.recents()
    },
    openTable(name: string) { this.trail = rootEntry(name) },
    follow(e: NavEntry) { this.trail = pushEntry(this.trail, e) },
    backTo(index: number) { this.trail = popTo(this.trail, index) },
  },
})
```

- [ ] **Step 6: Componentes do shell**

`frontend/src/components/EmptyState.vue`:

```vue
<script setup lang="ts">
import { Button } from "@/components/ui/button"
import { useViewer } from "@/stores/viewer"
const store = useViewer()
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center gap-6 p-8">
    <div class="text-center">
      <h1 class="text-2xl font-semibold">SQLite Viewer</h1>
      <p class="text-muted-foreground mt-1">Abra um arquivo de banco para começar.</p>
    </div>
    <Button @click="store.openDialog()">Abrir banco…</Button>
    <div v-if="store.recents.length" class="w-full max-w-md">
      <p class="text-muted-foreground mb-2 text-sm">Recentes</p>
      <ul class="space-y-1">
        <li v-for="p in store.recents" :key="p">
          <button class="hover:bg-accent w-full truncate rounded px-2 py-1 text-left text-sm" :title="p" @click="store.openPath(p)">
            {{ p }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>
```

`frontend/src/components/AppSidebar.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from "vue"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { useViewer } from "@/stores/viewer"

const store = useViewer()
const search = ref("")
const visible = computed(() =>
  store.tables.filter((t) => t.name.toLowerCase().includes(search.value.toLowerCase())),
)
</script>

<template>
  <aside class="bg-muted/30 flex h-full w-64 shrink-0 flex-col border-r">
    <div class="flex items-center gap-2 border-b p-3">
      <div class="min-w-0 flex-1">
        <p class="truncate text-sm font-medium" :title="store.db?.path">{{ store.db?.name ?? "Nenhum banco" }}</p>
        <p v-if="store.db?.readOnly" class="text-muted-foreground text-xs">somente leitura</p>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger as-child><Button variant="outline" size="sm">Abrir</Button></DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-72">
          <DropdownMenuItem @click="store.openDialog()">Abrir arquivo…</DropdownMenuItem>
          <DropdownMenuItem v-for="p in store.recents" :key="p" class="truncate" @click="store.openPath(p)">{{ p }}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    <div class="p-2"><Input v-model="search" placeholder="Buscar tabela…" /></div>
    <ScrollArea class="flex-1">
      <ul class="space-y-0.5 p-2">
        <li v-for="t in visible" :key="t.name">
          <button
            class="hover:bg-accent flex w-full items-center gap-2 rounded px-2 py-1 text-left text-sm"
            :class="{ 'bg-accent': store.trail[0]?.table === t.name }"
            @click="store.openTable(t.name)"
          >
            <span class="text-muted-foreground w-4 text-center text-xs">{{ t.kind === "view" ? "◇" : "▦" }}</span>
            <span class="truncate">{{ t.name }}</span>
          </button>
        </li>
      </ul>
    </ScrollArea>
  </aside>
</template>
```

`frontend/src/App.vue` (por enquanto sem abas; `DataTab`/`StructureTab` entram nas Tasks 11–12):

```vue
<script setup lang="ts">
import { onMounted } from "vue"
import { Sonner } from "@/components/ui/sonner"
import AppSidebar from "@/components/AppSidebar.vue"
import EmptyState from "@/components/EmptyState.vue"
import { useViewer } from "@/stores/viewer"

const store = useViewer()
onMounted(() => store.init())
</script>

<template>
  <div class="bg-background text-foreground flex h-screen">
    <EmptyState v-if="!store.db" class="flex-1" />
    <template v-else>
      <AppSidebar />
      <main class="min-w-0 flex-1 overflow-hidden">
        <p v-if="!store.current" class="text-muted-foreground p-8">Escolha uma tabela na barra lateral.</p>
        <p v-else class="p-8">{{ store.current.label }}</p>
      </main>
    </template>
    <Sonner />
  </div>
</template>
```

- [ ] **Step 7: Verificar**

Run (em `frontend/`): `npm test && npm run build`
Expected: PASS e build verde.
Run (na raiz): `wails3 dev` e conferir à mão: estado vazio, "Abrir banco…" abre o seletor nativo, abrir `testdata/sample.db` lista as tabelas na sidebar (views com ícone `◇`), clicar numa tabela mostra seu nome; fechar e abrir de novo reabre o último banco; `bin/sqliteviewer testdata/sample.db` (após `wails3 build`) abre direto o banco.

- [ ] **Step 8: Commit**

```bash
git add -A && git commit -m "feat(frontend): app shell, sidebar, recents and navigation trail"
```

---

### Task 11: Aba Dados (grid, paginação, ordenação, filtro, navegação por FK)

**Files:**
- Create: `frontend/src/components/CellValue.vue`, `frontend/src/components/DataGrid.vue`, `frontend/src/components/DataTab.vue`
- Modify: `frontend/src/App.vue`

**Interfaces:**
- Consumes: `api.getTable`, `api.queryRows`, `outgoingTarget`, `useViewer().follow`, tipos
- Produces:
  - `CellValue` props `{value: unknown; fk?: ForeignKey}`, emite `follow(fk)`
  - `DataGrid` props `{schema: TableSchema; rows: Row[]; orderBy: string; desc: boolean}`, emite `sort(column)`, `select(row)`, `follow(fk, row)`
  - `DataTab` props `{entry: NavEntry}`; expõe o painel via `RowSheet` (Task 12 — nesta task o clique na linha só guarda `selected`)

- [ ] **Step 1: `CellValue.vue`**

```vue
<script setup lang="ts">
import { computed } from "vue"
import { isBlob, type ForeignKey } from "@/lib/types"

const props = defineProps<{ value: unknown; fk?: ForeignKey }>()
const emit = defineEmits<{ follow: [fk: ForeignKey] }>()
const text = computed(() => String(props.value))
</script>

<template>
  <span v-if="value === null || value === undefined" class="text-muted-foreground italic">NULL</span>
  <span v-else-if="isBlob(value)" class="text-muted-foreground">&lt;blob {{ value.$blob }} bytes&gt;</span>
  <button
    v-else-if="fk"
    class="text-primary max-w-xs truncate underline underline-offset-2"
    :title="`Ir para ${fk.table}`"
    @click.stop="emit('follow', fk)"
  >{{ text }}</button>
  <span v-else-if="value === ''" class="text-muted-foreground italic">(vazio)</span>
  <span v-else class="block max-w-xs truncate" :title="text">{{ text }}</span>
</template>
```

- [ ] **Step 2: `DataGrid.vue` (TanStack Table, paginação e ordenação no servidor)**

```vue
<script setup lang="ts">
import { computed, h } from "vue"
import { FlexRender, getCoreRowModel, useVueTable, type ColumnDef } from "@tanstack/vue-table"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import CellValue from "./CellValue.vue"
import type { ForeignKey, Row, TableSchema } from "@/lib/types"

const props = defineProps<{ schema: TableSchema; rows: Row[]; orderBy: string; desc: boolean }>()
const emit = defineEmits<{ sort: [column: string]; select: [row: Row]; follow: [fk: ForeignKey, row: Row] }>()

function fkFor(column: string): ForeignKey | undefined {
  return props.schema.foreignKeys.find((fk) => fk.from.includes(column))
}

const columns = computed<ColumnDef<Row>[]>(() =>
  props.schema.columns.map((c) => ({
    id: c.name,
    accessorFn: (r) => r.values[c.name],
    header: () =>
      h(
        "button",
        { class: "flex items-center gap-1 font-medium", onClick: () => emit("sort", c.name) },
        [c.name, props.orderBy === c.name ? (props.desc ? " ↓" : " ↑") : ""],
      ),
    cell: ({ row }) =>
      h(CellValue, {
        value: row.original.values[c.name],
        fk: fkFor(c.name),
        onFollow: (fk: ForeignKey) => emit("follow", fk, row.original),
      }),
  })),
)

const table = useVueTable({
  get data() { return props.rows },
  get columns() { return columns.value },
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualSorting: true,
})
</script>

<template>
  <Table>
    <TableHeader>
      <TableRow v-for="hg in table.getHeaderGroups()" :key="hg.id">
        <TableHead v-for="header in hg.headers" :key="header.id">
          <FlexRender v-if="!header.isPlaceholder" :render="header.column.columnDef.header" :props="header.getContext()" />
        </TableHead>
      </TableRow>
    </TableHeader>
    <TableBody>
      <TableRow v-for="row in table.getRowModel().rows" :key="row.id" class="cursor-pointer" @click="emit('select', row.original)">
        <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
          <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
        </TableCell>
      </TableRow>
      <TableRow v-if="!rows.length">
        <TableCell :colspan="schema.columns.length" class="text-muted-foreground py-8 text-center">Nenhum registro.</TableCell>
      </TableRow>
    </TableBody>
  </Table>
</template>
```

- [ ] **Step 3: `DataTab.vue`**

```vue
<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { toast } from "vue-sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import DataGrid from "./DataGrid.vue"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { outgoingTarget, type NavEntry } from "@/lib/nav"
import type { ForeignKey, Page, Row, TableSchema } from "@/lib/types"
import { useViewer } from "@/stores/viewer"

const props = defineProps<{ entry: NavEntry }>()
const store = useViewer()

const schema = ref<TableSchema | null>(null)
const page = ref<Page | null>(null)
const pageNo = ref(1)
const pageSize = 50
const orderBy = ref("")
const desc = ref(false)
const filter = ref("")
const selected = ref<Row | null>(null) // used by the row sheet (Task 12)
const loading = ref(false)

const pages = computed(() => Math.max(1, Math.ceil((page.value?.total ?? 0) / pageSize)))

async function load() {
  loading.value = true
  try {
    if (!schema.value || schema.value.name !== props.entry.table) schema.value = await api.getTable(props.entry.table)
    page.value = await api.queryRows({
      table: props.entry.table, page: pageNo.value, pageSize,
      orderBy: orderBy.value, desc: desc.value, filter: filter.value, where: props.entry.where,
    })
  } catch (e) {
    toast.error(toAppError(e).message)
  } finally {
    loading.value = false
  }
}

let timer: ReturnType<typeof setTimeout>
watch(filter, () => { clearTimeout(timer); timer = setTimeout(() => { pageNo.value = 1; load() }, 250) })
watch(() => props.entry, () => { pageNo.value = 1; orderBy.value = ""; filter.value = ""; load() }, { immediate: true })

function sort(column: string) {
  if (orderBy.value === column) desc.value = !desc.value
  else { orderBy.value = column; desc.value = false }
  pageNo.value = 1
  load()
}
function go(n: number) { pageNo.value = Math.min(pages.value, Math.max(1, n)); load() }
function follow(fk: ForeignKey, row: Row) { store.follow(outgoingTarget(fk, row.values)) }

defineExpose({ reload: load, selected })
</script>

<template>
  <div v-if="schema && page" class="flex h-full flex-col">
    <div class="flex items-center gap-2 border-b p-3">
      <Input v-model="filter" placeholder="Filtrar texto…" class="max-w-xs" />
      <span class="text-muted-foreground text-sm">{{ page.total }} registros</span>
      <div class="flex-1" />
      <Button size="sm" variant="outline" :disabled="pageNo <= 1" @click="go(pageNo - 1)">‹</Button>
      <span class="text-sm">{{ pageNo }} / {{ pages }}</span>
      <Button size="sm" variant="outline" :disabled="pageNo >= pages" @click="go(pageNo + 1)">›</Button>
      <Button v-if="!schema.readOnly && !store.db?.readOnly" size="sm" @click="selected = null">Novo</Button>
    </div>
    <div class="min-h-0 flex-1 overflow-auto" :class="{ 'opacity-60': loading }">
      <DataGrid :schema="schema" :rows="page.rows" :order-by="orderBy" :desc="desc"
        @sort="sort" @select="selected = $event" @follow="follow" />
    </div>
  </div>
</template>
```

- [ ] **Step 4: Ligar no `App.vue` com breadcrumb**

Substituir o `<main>` do `App.vue`. Adicionar imports de `DataTab` e dos componentes `Breadcrumb*` e `Tabs*`; a aba "Estrutura" entra na Task 12 (deixar `TabsContent value="structure"` com um texto provisório).

```vue
<main class="flex min-w-0 flex-1 flex-col overflow-hidden">
  <p v-if="!store.current" class="text-muted-foreground p-8">Escolha uma tabela na barra lateral.</p>
  <template v-else>
    <Breadcrumb class="border-b px-4 py-2">
      <BreadcrumbList>
        <template v-for="(e, i) in store.trail" :key="i">
          <BreadcrumbSeparator v-if="i > 0" />
          <BreadcrumbItem>
            <BreadcrumbPage v-if="i === store.trail.length - 1">{{ e.label }}</BreadcrumbPage>
            <BreadcrumbLink v-else as="button" @click="store.backTo(i)">{{ e.label }}</BreadcrumbLink>
          </BreadcrumbItem>
        </template>
      </BreadcrumbList>
    </Breadcrumb>
    <Tabs default-value="data" class="flex min-h-0 flex-1 flex-col">
      <TabsList class="mx-4 mt-2 w-fit">
        <TabsTrigger value="data">Dados</TabsTrigger>
        <TabsTrigger value="structure">Estrutura</TabsTrigger>
      </TabsList>
      <TabsContent value="data" class="min-h-0 flex-1">
        <DataTab :key="store.trail.length + store.current.label" :entry="store.current" />
      </TabsContent>
      <TabsContent value="structure" class="min-h-0 flex-1 overflow-auto">Estrutura (Task 12)</TabsContent>
    </Tabs>
  </template>
</main>
```

- [ ] **Step 5: Verificar**

Run (em `frontend/`): `npm run build`
Expected: build verde.
Run (na raiz): `wails3 dev`; com `testdata/sample.db`: abrir `posts` → grid com 4 linhas; clicar no cabeçalho ordena (↑/↓); digitar `bru` no filtro mostra só o post do Bruno... (em `users`); `user_id` aparece como link; clicar leva a `users (id = 1)` com breadcrumb e voltar pelo breadcrumb funciona; `blobs` mostra `<blob 4 bytes>` e o inteiro grande como `9007199254740993`; `v_posts` não mostra botão "Novo"; `NULL` aparece em itálico.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat(frontend): data tab with grid, paging, sorting, filter and FK navigation"
```

---

### Task 12: Painel lateral com formulário (insert/update/delete) e aba Estrutura

**Files:**
- Create: `frontend/src/lib/formValues.ts`, `frontend/src/lib/useRowForm.ts`, `frontend/src/components/RowSheet.vue`, `frontend/src/components/StructureTab.vue`
- Modify: `frontend/src/components/DataTab.vue`, `frontend/src/App.vue`
- Test: `frontend/src/lib/formValues.test.ts`, `frontend/src/lib/useRowForm.test.ts`

**Interfaces:**
- Consumes: `inputKind`, `api`, `AppError`/`toAppError`, tipos
- Produces:
  - `lib/formValues.ts`:
    - `FieldMode = "value" | "null" | "default"`; `FieldState{mode: FieldMode; text: string; bool: boolean}`; `FormState = Record<string, FieldState>`
    - `initialState(cols: Column[], row: Row | null): FormState` — insert (`row=null`): todo campo em `default`; edição: `value`, ou `null` se o valor for NULL; colunas geradas ficam de fora
    - `toPayload(cols: Column[], form: FormState, original: Values | null): { values: Values; errors: Record<string, string> }` — insert omite campos em `default`; edição envia só campos que mudaram; `null` → `null`; texto vazio em coluna de texto → `""`; inteiro/número vazio ou inválido → erro de campo; inteiro fora do range seguro vai como string; booleano → `1`/`0`; BLOB nunca é enviado
    - `splitErrors(err: AppError, cols: Column[]): { fields: Record<string, string>; general: string }`
  - `lib/useRowForm.ts`: `useRowForm(getTable: () => TableSchema, getRow: () => Row | null)` retorna `{ form, fieldErrors, generalError, saving, reset(), submit(): Promise<boolean>, remove(): Promise<boolean> }`
  - `RowSheet` props `{open: boolean; table: TableSchema; row: Row | null; readOnly: boolean}`, emite `update:open`, `saved`, `deleted`

- [ ] **Step 1: Escrever os testes que falham**

`frontend/src/lib/formValues.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { initialState, splitErrors, toPayload } from "./formValues";
import type { Column } from "./types";

const col = (name: string, type: string, extra: Partial<Column> = {}): Column =>
  ({ name, type, notNull: false, default: null, pk: 0, generated: false, ...extra });

const cols = [col("id", "INTEGER", { pk: 1 }), col("name", "TEXT"), col("age", "INTEGER"), col("score", "REAL"), col("ok", "BOOLEAN")];

describe("initialState", () => {
  it("insert: every field starts in default mode", () => {
    const f = initialState(cols, null);
    expect(Object.values(f).every((s) => s.mode === "default")).toBe(true);
  });
  it("edit: NULL becomes null mode, values become value mode", () => {
    const f = initialState(cols, { key: { id: 1 }, values: { id: 1, name: null, age: 30, score: 1.5, ok: 1 } });
    expect(f.name.mode).toBe("null");
    expect(f.age).toMatchObject({ mode: "value", text: "30" });
    expect(f.ok.bool).toBe(true);
  });
  it("skips generated columns", () => {
    expect("g" in initialState([col("g", "INTEGER", { generated: true })], null)).toBe(false);
  });
});

describe("toPayload — insert", () => {
  it("omits default-mode fields", () => {
    const f = initialState(cols, null);
    f.name = { mode: "value", text: "Ana", bool: false };
    expect(toPayload(cols, f, null)).toEqual({ values: { name: "Ana" }, errors: {} });
  });
  it("distinguishes NULL from empty string", () => {
    const f = initialState(cols, null);
    f.name = { mode: "null", text: "", bool: false };
    expect(toPayload(cols, f, null).values).toEqual({ name: null });
    f.name = { mode: "value", text: "", bool: false };
    expect(toPayload(cols, f, null).values).toEqual({ name: "" });
  });
  it("parses numbers and booleans", () => {
    const f = initialState(cols, null);
    f.age = { mode: "value", text: "42", bool: false };
    f.score = { mode: "value", text: "2.5", bool: false };
    f.ok = { mode: "value", text: "", bool: true };
    expect(toPayload(cols, f, null).values).toEqual({ age: 42, score: 2.5, ok: 1 });
  });
  it("reports empty or invalid numeric input as a field error", () => {
    const f = initialState(cols, null);
    f.age = { mode: "value", text: "", bool: false };
    f.score = { mode: "value", text: "abc", bool: false };
    const { errors, values } = toPayload(cols, f, null);
    expect(Object.keys(errors).sort()).toEqual(["age", "score"]);
    expect(values).toEqual({});
  });
  it("keeps integers beyond 2^53 as exact strings", () => {
    const f = initialState(cols, null);
    f.age = { mode: "value", text: "9007199254740993", bool: false };
    expect(toPayload(cols, f, null).values.age).toBe("9007199254740993");
  });
});

describe("toPayload — edit", () => {
  const row = { key: { id: 1 }, values: { id: 1, name: "Ana", age: 30, score: 1.5, ok: 0 } };
  it("sends only changed fields", () => {
    const f = initialState(cols, row);
    expect(toPayload(cols, f, row.values).values).toEqual({});
    f.name.text = "Ana Maria";
    expect(toPayload(cols, f, row.values).values).toEqual({ name: "Ana Maria" });
  });
  it("can change a value to NULL", () => {
    const f = initialState(cols, row);
    f.name.mode = "null";
    expect(toPayload(cols, f, row.values).values).toEqual({ name: null });
  });
  it("never sends blob columns", () => {
    const blobCols = [col("id", "INTEGER", { pk: 1 }), col("data", "BLOB")];
    const r = { key: { id: 1 }, values: { id: 1, data: { $blob: 4 } } };
    const f = initialState(blobCols, r);
    expect(toPayload(blobCols, f, r.values).values).toEqual({});
  });
});

describe("splitErrors", () => {
  it("assigns constraint errors to the matching fields", () => {
    const r = splitErrors({ message: "email é obrigatório", constraint: "NOT NULL", columns: ["name"] }, cols);
    expect(r.fields).toEqual({ name: "email é obrigatório" });
    expect(r.general).toBe("");
  });
  it("uses a general message when no column matches", () => {
    const r = splitErrors({ message: "FK falhou", constraint: "FOREIGN KEY", columns: [] }, cols);
    expect(r.fields).toEqual({});
    expect(r.general).toBe("FK falhou");
  });
});
```

`frontend/src/lib/useRowForm.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api", () => ({
  api: { insertRow: vi.fn(), updateRow: vi.fn(), deleteRow: vi.fn() },
}));

import { api } from "@/lib/api";
import { useRowForm } from "./useRowForm";
import type { TableSchema } from "./types";

const table: TableSchema = {
  name: "users", kind: "table", readOnly: false, primaryKey: ["id"], usesRowId: false, keyColumns: ["id"],
  foreignKeys: [], incoming: [],
  columns: [
    { name: "id", type: "INTEGER", notNull: false, default: null, pk: 1, generated: false },
    { name: "email", type: "TEXT", notNull: true, default: null, pk: 0, generated: false },
  ],
};
const row = { key: { id: 1 }, values: { id: 1, email: "a@x.com" } };

beforeEach(() => vi.clearAllMocks());

describe("useRowForm", () => {
  it("insert calls insertRow with only provided fields", async () => {
    (api.insertRow as any).mockResolvedValue({ id: 2 });
    const f = useRowForm(() => table, () => null);
    f.reset();
    f.form.value.email = { mode: "value", text: "b@x.com", bool: false };
    expect(await f.submit()).toBe(true);
    expect(api.insertRow).toHaveBeenCalledWith("users", { email: "b@x.com" });
  });

  it("edit with no changes does not call the backend", async () => {
    const f = useRowForm(() => table, () => row);
    f.reset();
    expect(await f.submit()).toBe(true);
    expect(api.updateRow).not.toHaveBeenCalled();
  });

  it("edit sends the key and changed fields", async () => {
    (api.updateRow as any).mockResolvedValue(undefined);
    const f = useRowForm(() => table, () => row);
    f.reset();
    f.form.value.email.text = "z@x.com";
    await f.submit();
    expect(api.updateRow).toHaveBeenCalledWith("users", { id: 1 }, { email: "z@x.com" });
  });

  it("maps a constraint error onto the right field and keeps the form open", async () => {
    (api.insertRow as any).mockRejectedValue(
      Object.assign(new Error("x"), { cause: { type: "constraint", kind: "UNIQUE", columns: ["email"], message: "já existe" } }),
    );
    const f = useRowForm(() => table, () => null);
    f.reset();
    f.form.value.email = { mode: "value", text: "a@x.com", bool: false };
    expect(await f.submit()).toBe(false);
    expect(f.fieldErrors.value).toEqual({ email: "já existe" });
    expect(f.saving.value).toBe(false);
  });

  it("client-side validation errors never reach the backend", async () => {
    const t2 = { ...table, columns: [...table.columns, { name: "age", type: "INTEGER", notNull: false, default: null, pk: 0, generated: false }] };
    const f = useRowForm(() => t2, () => null);
    f.reset();
    f.form.value.age = { mode: "value", text: "abc", bool: false };
    expect(await f.submit()).toBe(false);
    expect(api.insertRow).not.toHaveBeenCalled();
    expect(f.fieldErrors.value.age).toBeTruthy();
  });

  it("remove deletes by key and reports general errors", async () => {
    (api.deleteRow as any).mockRejectedValue(
      Object.assign(new Error("x"), { cause: { type: "constraint", kind: "FOREIGN KEY", columns: [], message: "há dependentes" } }),
    );
    const f = useRowForm(() => table, () => row);
    f.reset();
    expect(await f.remove()).toBe(false);
    expect(api.deleteRow).toHaveBeenCalledWith("users", { id: 1 });
    expect(f.generalError.value).toBe("há dependentes");
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run (em `frontend/`): `npm test -- formValues useRowForm`
Expected: FAIL (módulos inexistentes).

- [ ] **Step 3: Implementar `formValues.ts`**

```ts
import { inputKind } from "./columnKind"
import type { AppError } from "./errors"
import { isBlob, type Column, type Row, type Values } from "./types"

export type FieldMode = "value" | "null" | "default"
export interface FieldState { mode: FieldMode; text: string; bool: boolean }
export type FormState = Record<string, FieldState>

const editable = (c: Column) => !c.generated

export function initialState(cols: Column[], row: Row | null): FormState {
  const form: FormState = {}
  for (const c of cols.filter(editable)) {
    if (!row) { form[c.name] = { mode: "default", text: "", bool: false }; continue }
    const v = row.values[c.name]
    if (v === null || v === undefined) form[c.name] = { mode: "null", text: "", bool: false }
    else if (isBlob(v)) form[c.name] = { mode: "value", text: "", bool: false }
    else form[c.name] = { mode: "value", text: String(v), bool: v === 1 || v === true || v === "1" }
  }
  return form
}

function parse(c: Column, st: FieldState): { ok: true; value: unknown } | { ok: false; error: string } {
  const kind = inputKind(c.type)
  if (kind === "boolean") return { ok: true, value: st.bool ? 1 : 0 }
  if (kind === "integer") {
    const t = st.text.trim()
    if (!/^-?\d+$/.test(t)) return { ok: false, error: "informe um número inteiro" }
    const n = Number(t)
    return { ok: true, value: Number.isSafeInteger(n) ? n : t }
  }
  if (kind === "number") {
    const t = st.text.trim()
    const n = Number(t)
    if (t === "" || !Number.isFinite(n)) return { ok: false, error: "informe um número" }
    return { ok: true, value: n }
  }
  return { ok: true, value: st.text }
}

export function toPayload(cols: Column[], form: FormState, original: Values | null) {
  const values: Values = {}
  const errors: Record<string, string> = {}
  for (const c of cols.filter(editable)) {
    const st = form[c.name]
    if (!st) continue
    if (original && isBlob(original[c.name])) continue // blobs are never edited
    if (!original && st.mode === "default") continue
    let next: unknown
    if (st.mode === "null") next = null
    else {
      const p = parse(c, st)
      if (!p.ok) { errors[c.name] = p.error; continue }
      next = p.value
    }
    if (original) {
      const prev = original[c.name]
      const same = prev === next || (prev !== null && next !== null && String(prev) === String(next))
      if (same) continue
    }
    values[c.name] = next
  }
  return { values, errors }
}

export function splitErrors(err: AppError, cols: Column[]) {
  const fields: Record<string, string> = {}
  for (const name of err.columns) {
    if (cols.some((c) => c.name === name)) fields[name] = err.message
  }
  return { fields, general: Object.keys(fields).length ? "" : err.message }
}
```

Nota: em modo edição com campo em `default` não existe (a edição só usa `value`/`null`), então a condição `!original && mode==="default"` é suficiente.

- [ ] **Step 4: Implementar `useRowForm.ts`**

```ts
import { ref } from "vue"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { initialState, splitErrors, toPayload, type FormState } from "@/lib/formValues"
import type { Row, TableSchema } from "@/lib/types"

export function useRowForm(getTable: () => TableSchema, getRow: () => Row | null) {
  const form = ref<FormState>({})
  const fieldErrors = ref<Record<string, string>>({})
  const generalError = ref("")
  const saving = ref(false)

  function reset() {
    form.value = initialState(getTable().columns, getRow())
    fieldErrors.value = {}
    generalError.value = ""
  }

  function fail(e: unknown) {
    const r = splitErrors(toAppError(e), getTable().columns)
    fieldErrors.value = r.fields
    generalError.value = r.general
  }

  async function submit(): Promise<boolean> {
    const table = getTable()
    const row = getRow()
    const { values, errors } = toPayload(table.columns, form.value, row?.values ?? null)
    fieldErrors.value = errors
    generalError.value = ""
    if (Object.keys(errors).length) return false
    if (row && Object.keys(values).length === 0) return true // nothing changed
    saving.value = true
    try {
      if (row) await api.updateRow(table.name, row.key, values)
      else await api.insertRow(table.name, values)
      return true
    } catch (e) {
      fail(e)
      return false
    } finally {
      saving.value = false
    }
  }

  async function remove(): Promise<boolean> {
    const row = getRow()
    if (!row) return false
    saving.value = true
    generalError.value = ""
    try {
      await api.deleteRow(getTable().name, row.key)
      return true
    } catch (e) {
      fail(e)
      return false
    } finally {
      saving.value = false
    }
  }

  return { form, fieldErrors, generalError, saving, reset, submit, remove }
}
```

- [ ] **Step 5: Rodar e ver passar**

Run (em `frontend/`): `npm test`
Expected: PASS em todos os arquivos de teste.

- [ ] **Step 6: `RowSheet.vue`**

Usa `useRowForm`, `inputKind`, `Sheet`, `AlertDialog`, `Input`, `Textarea`, `Checkbox`, `Label`, `Button`. FK combobox e "Referenciado por" são plugados na Task 13 (aqui os campos de FK renderizam como input normal).

```vue
<script setup lang="ts">
import { computed, watch } from "vue"
import { toast } from "vue-sonner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Sheet, SheetContent, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { inputKind } from "@/lib/columnKind"
import { useRowForm } from "@/lib/useRowForm"
import { isBlob, type Row, type TableSchema } from "@/lib/types"

const props = defineProps<{ open: boolean; table: TableSchema; row: Row | null; readOnly: boolean }>()
const emit = defineEmits<{ "update:open": [v: boolean]; saved: []; deleted: [] }>()

const f = useRowForm(() => props.table, () => props.row)
watch(() => [props.open, props.table.name, props.row], () => { if (props.open) f.reset() }, { immediate: true })

const columns = computed(() => props.table.columns.filter((c) => !c.generated))
const title = computed(() => (props.row ? `Editar registro` : `Novo registro`))

async function save() {
  if (await f.submit()) { toast.success("Salvo"); emit("saved"); emit("update:open", false) }
}
async function del() {
  if (await f.remove()) { toast.success("Registro excluído"); emit("deleted"); emit("update:open", false) }
}
function onInput(name: string) {
  const st = f.form.value[name]
  if (st.mode !== "value") st.mode = "value"
}
</script>

<template>
  <Sheet :open="open" @update:open="emit('update:open', $event)">
    <SheetContent class="flex w-[440px] flex-col gap-0 overflow-y-auto sm:max-w-[440px]">
      <SheetHeader><SheetTitle>{{ title }} — {{ table.name }}</SheetTitle></SheetHeader>

      <form class="flex-1 space-y-4 px-4 pb-4" @submit.prevent="save">
        <div v-for="c in columns" :key="c.name" class="space-y-1">
          <div class="flex items-center justify-between">
            <Label :for="`f-${c.name}`">
              {{ c.name }}
              <span class="text-muted-foreground text-xs">{{ c.type }}<template v-if="c.notNull"> · obrigatório</template><template v-if="c.pk"> · PK</template></span>
            </Label>
            <Button v-if="!c.notNull && !readOnly && !(row && isBlob(row.values[c.name]))" type="button" variant="ghost" size="sm"
              :class="{ 'text-primary': f.form.value[c.name]?.mode === 'null' }"
              @click="f.form.value[c.name].mode = f.form.value[c.name].mode === 'null' ? 'value' : 'null'">NULL</Button>
          </div>

          <p v-if="row && isBlob(row.values[c.name])" class="text-muted-foreground text-sm">&lt;blob {{ (row.values[c.name] as any).$blob }} bytes&gt; (não editável)</p>
          <template v-else-if="f.form.value[c.name]">
            <div v-if="inputKind(c.type) === 'boolean'" class="flex items-center gap-2">
              <Checkbox :id="`f-${c.name}`" :model-value="f.form.value[c.name].bool" :disabled="readOnly || f.form.value[c.name].mode === 'null'"
                @update:model-value="(v: boolean | 'indeterminate') => { f.form.value[c.name].bool = v === true; onInput(c.name) }" />
            </div>
            <Textarea v-else-if="inputKind(c.type) === 'textarea'" :id="`f-${c.name}`" v-model="f.form.value[c.name].text" rows="3"
              :disabled="readOnly || f.form.value[c.name].mode === 'null'"
              :placeholder="f.form.value[c.name].mode === 'default' ? (c.default ?? 'padrão') : ''" @input="onInput(c.name)" />
            <Input v-else :id="`f-${c.name}`" v-model="f.form.value[c.name].text"
              :type="inputKind(c.type) === 'integer' || inputKind(c.type) === 'number' ? 'text' : 'text'"
              :inputmode="inputKind(c.type) === 'integer' ? 'numeric' : inputKind(c.type) === 'number' ? 'decimal' : 'text'"
              :disabled="readOnly || f.form.value[c.name].mode === 'null'"
              :placeholder="f.form.value[c.name].mode === 'default' ? (c.default ?? (inputKind(c.type) === 'date' ? 'AAAA-MM-DD' : inputKind(c.type) === 'datetime' ? 'AAAA-MM-DD HH:MM:SS' : 'padrão')) : ''"
              @input="onInput(c.name)" />
            <p v-if="f.form.value[c.name].mode === 'null'" class="text-muted-foreground text-xs">Será gravado como NULL.</p>
          </template>
          <p v-if="f.fieldErrors.value[c.name]" class="text-destructive text-sm">{{ f.fieldErrors.value[c.name] }}</p>
        </div>

        <p v-if="f.generalError.value" class="text-destructive text-sm">{{ f.generalError.value }}</p>
        <slot name="extra" />
      </form>

      <SheetFooter class="flex-row justify-between border-t p-4">
        <AlertDialog v-if="row && !readOnly">
          <AlertDialogTrigger as-child><Button variant="destructive" :disabled="f.saving.value">Excluir</Button></AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Excluir este registro?</AlertDialogTitle>
              <AlertDialogDescription>Essa ação não pode ser desfeita.</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancelar</AlertDialogCancel>
              <AlertDialogAction @click="del">Excluir</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        <span v-else />
        <div class="flex gap-2">
          <Button variant="outline" @click="emit('update:open', false)">{{ readOnly ? "Fechar" : "Cancelar" }}</Button>
          <Button v-if="!readOnly" :disabled="f.saving.value" @click="save">Salvar</Button>
        </div>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>
```

- [ ] **Step 7: Plugar o painel no `DataTab.vue`**

Em `DataTab.vue`: trocar `selected` por dois estados — `sheetOpen = ref(false)` e `selected = ref<Row | null>(null)`; "Novo" faz `selected = null; sheetOpen = true`; `@select` da grid faz `selected = row; sheetOpen = true`. Calcular `const readOnly = computed(() => !!schema.value?.readOnly || !!store.db?.readOnly)`. No template, após o grid:

```vue
<RowSheet v-model:open="sheetOpen" :table="schema" :row="selected" :read-only="readOnly"
  @saved="load" @deleted="load" />
```

e importar `RowSheet`. O botão "Novo" fica com `v-if="!readOnly"`.

- [ ] **Step 8: `StructureTab.vue`**

```vue
<script setup lang="ts">
import { ref, watch } from "vue"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api } from "@/lib/api"
import type { NavEntry } from "@/lib/nav"
import type { TableSchema } from "@/lib/types"
import { useViewer } from "@/stores/viewer"

const props = defineProps<{ table: string }>()
const store = useViewer()
const schema = ref<TableSchema | null>(null)
watch(() => props.table, async (t) => { schema.value = await api.getTable(t) }, { immediate: true })

const go = (table: string) => store.follow({ table, where: [], label: table } as NavEntry)
</script>

<template>
  <div v-if="schema" class="space-y-6 p-4">
    <section>
      <h3 class="mb-2 font-medium">Colunas</h3>
      <Table>
        <TableHeader><TableRow>
          <TableHead>Nome</TableHead><TableHead>Tipo</TableHead><TableHead>Obrigatório</TableHead><TableHead>Padrão</TableHead><TableHead />
        </TableRow></TableHeader>
        <TableBody>
          <TableRow v-for="c in schema.columns" :key="c.name">
            <TableCell class="font-mono">{{ c.name }}</TableCell>
            <TableCell>{{ c.type || "—" }}</TableCell>
            <TableCell>{{ c.notNull ? "sim" : "não" }}</TableCell>
            <TableCell class="font-mono">{{ c.default ?? "—" }}</TableCell>
            <TableCell class="space-x-1">
              <Badge v-if="c.pk" variant="secondary">PK</Badge>
              <Badge v-if="c.generated" variant="outline">gerada</Badge>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <p v-if="schema.usesRowId" class="text-muted-foreground mt-2 text-sm">Sem chave primária: linhas identificadas por rowid.</p>
      <p v-if="schema.readOnly" class="text-muted-foreground mt-2 text-sm">Somente leitura.</p>
    </section>

    <section>
      <h3 class="mb-2 font-medium">Referencia (saída)</h3>
      <p v-if="!schema.foreignKeys.length" class="text-muted-foreground text-sm">Nenhuma chave estrangeira.</p>
      <ul class="space-y-1">
        <li v-for="(fk, i) in schema.foreignKeys" :key="i" class="text-sm">
          <span class="font-mono">{{ fk.from.join(", ") }}</span> →
          <button class="text-primary underline" @click="go(fk.table)">{{ fk.table }}</button>
          (<span class="font-mono">{{ fk.to.join(", ") }}</span>)
          <span class="text-muted-foreground"> · ON DELETE {{ fk.onDelete }}</span>
        </li>
      </ul>
    </section>

    <section>
      <h3 class="mb-2 font-medium">Referenciada por (entrada)</h3>
      <p v-if="!schema.incoming.length" class="text-muted-foreground text-sm">Nenhuma tabela referencia esta.</p>
      <ul class="space-y-1">
        <li v-for="(fk, i) in schema.incoming" :key="i" class="text-sm">
          <button class="text-primary underline" @click="go(fk.table)">{{ fk.table }}</button>
          (<span class="font-mono">{{ fk.from.join(", ") }}</span>) → <span class="font-mono">{{ fk.to.join(", ") }}</span>
        </li>
      </ul>
    </section>
  </div>
</template>
```

Em `App.vue`, substituir o texto provisório por `<StructureTab :table="store.current.table" />` (com import).

- [ ] **Step 9: Verificar**

Run (em `frontend/`): `npm test && npm run build`
Expected: PASS e build verde.
Run (na raiz): `wails3 dev` e conferir com `testdata/sample.db`:
- `users` → "Novo": preencher só `email` → salva, linha aparece; deixar `email` vazio → erro "email é obrigatório" no campo; repetir um e-mail → erro de unicidade no campo `email`.
- Editar `name` para NULL (botão NULL) e para vazio: grid mostra `NULL` vs `(vazio)`.
- `age`/campo numérico com "abc" → erro no campo, nada vai ao banco.
- Excluir um `users` com posts → erro geral "ainda há registros que dependem deste"; excluir um sem dependentes → some da grid.
- `v_posts` e abrir um banco somente leitura (`chmod 444` numa cópia) → sem "Novo"/"Excluir"/"Salvar".
- Aba Estrutura de `posts` mostra a FK para `users`; clicar navega.

- [ ] **Step 10: Commit**

```bash
git add -A && git commit -m "feat(frontend): row form sheet with insert/update/delete and structure tab"
```

---

### Task 13: FK combobox e "Referenciado por"

**Files:**
- Create: `frontend/src/components/FkCombobox.vue`, `frontend/src/components/ReferencedBy.vue`
- Modify: `frontend/src/components/RowSheet.vue`

**Interfaces:**
- Consumes: `api.queryRows`, `api.references`, `incomingTarget`, `useViewer().follow`
- Produces:
  - `FkCombobox` props `{fk: ForeignKey; modelValue: string; disabled?: boolean}`, emite `update:modelValue(text)` — só para FK de **uma** coluna; FK composta continua com inputs normais
  - `ReferencedBy` props `{table: string; row: Row; incoming: IncomingFK[]}`, emite `navigate` depois de seguir um link (para fechar o painel)

- [ ] **Step 1: `FkCombobox.vue`**

```vue
<script setup lang="ts">
import { ref, watch } from "vue"
import { Button } from "@/components/ui/button"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { api } from "@/lib/api"
import type { ForeignKey, Row, TableSchema } from "@/lib/types"

const props = defineProps<{ fk: ForeignKey; modelValue: string; disabled?: boolean }>()
const emit = defineEmits<{ "update:modelValue": [v: string] }>()

const open = ref(false)
const search = ref("")
const options = ref<Row[]>([])
const target = ref<TableSchema | null>(null)
const keyCol = props.fk.to[0]

async function load() {
  if (!target.value) target.value = await api.getTable(props.fk.table)
  const page = await api.queryRows({ table: props.fk.table, page: 1, pageSize: 20, orderBy: keyCol, desc: false, filter: search.value, where: [] })
  options.value = page.rows
}

function label(r: Row): string {
  const hint = target.value?.columns.find((c) => c.name !== keyCol && /CHAR|TEXT|CLOB/i.test(c.type))
  const extra = hint ? r.values[hint.name] : null
  return extra ? `${r.values[keyCol]} — ${extra}` : String(r.values[keyCol])
}

let timer: ReturnType<typeof setTimeout>
watch(search, () => { clearTimeout(timer); timer = setTimeout(load, 200) })
watch(open, (o) => { if (o) load() })

function pick(r: Row) {
  emit("update:modelValue", String(r.values[keyCol]))
  open.value = false
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button type="button" variant="outline" class="w-full justify-between font-normal" :disabled="disabled">
        <span class="truncate">{{ modelValue || `Escolher em ${fk.table}…` }}</span>
        <span class="text-muted-foreground">▾</span>
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-80 p-0" align="start">
      <Command :filter-function="(list: any) => list">
        <CommandInput v-model="search" placeholder="Buscar…" />
        <CommandList>
          <CommandEmpty>Nada encontrado.</CommandEmpty>
          <CommandGroup>
            <CommandItem v-for="r in options" :key="String(r.values[keyCol])" :value="String(r.values[keyCol])" @select="pick(r)">
              {{ label(r) }}
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </Command>
    </PopoverContent>
  </Popover>
</template>
```

(`:filter-function` devolve a lista sem filtrar no cliente, porque a busca é feita no servidor via `filter`.)

- [ ] **Step 2: `ReferencedBy.vue`**

```vue
<script setup lang="ts">
import { ref, watch } from "vue"
import { api } from "@/lib/api"
import { incomingTarget } from "@/lib/nav"
import type { IncomingFK, RefCount, Row } from "@/lib/types"
import { useViewer } from "@/stores/viewer"

const props = defineProps<{ table: string; row: Row; incoming: IncomingFK[] }>()
const emit = defineEmits<{ navigate: [] }>()
const store = useViewer()
const refs = ref<RefCount[]>([])

watch(() => [props.table, props.row], async () => {
  refs.value = props.incoming.length ? await api.references(props.table, props.row.key) : []
}, { immediate: true })

function go(r: RefCount) {
  store.follow(incomingTarget(r, props.row.values))
  emit("navigate")
}
</script>

<template>
  <section v-if="refs.length" class="border-t pt-4">
    <h4 class="mb-2 text-sm font-medium">Referenciado por</h4>
    <ul class="space-y-1">
      <li v-for="(r, i) in refs" :key="i" class="text-sm">
        <button class="text-primary underline" @click="go(r)">{{ r.table }}</button>
        <span class="text-muted-foreground"> · {{ r.count }} {{ r.count === 1 ? "registro" : "registros" }} (via {{ r.from.join(", ") }})</span>
      </li>
    </ul>
  </section>
</template>
```

- [ ] **Step 3: Plugar no `RowSheet.vue`**

1. Importar `FkCombobox` e `ReferencedBy`.
2. Função auxiliar: `const singleFk = (name: string) => props.table.foreignKeys.find((fk) => fk.from.length === 1 && fk.from[0] === name && fk.to[0])`.
3. No `v-else-if="f.form.value[c.name]"`, antes do ramo `inputKind === 'boolean'`, adicionar o ramo:

```vue
<FkCombobox v-if="singleFk(c.name) && f.form.value[c.name].mode !== 'null'" :fk="singleFk(c.name)!"
  :model-value="f.form.value[c.name].text" :disabled="readOnly"
  @update:model-value="(v: string) => { f.form.value[c.name].text = v; f.form.value[c.name].mode = 'value' }" />
```
   e transformar o ramo seguinte (`boolean`) de `v-if` em `v-else-if`.
4. Substituir `<slot name="extra" />` por:

```vue
<ReferencedBy v-if="row" :table="table.name" :row="row" :incoming="table.incoming" @navigate="emit('update:open', false)" />
```

- [ ] **Step 4: Verificar**

Run (em `frontend/`): `npm test && npm run build`
Expected: PASS e build verde.
Run (na raiz): `wails3 dev` com `testdata/sample.db`:
- `posts` → "Novo": o campo `user_id` abre um combobox com busca (`ana`, `bru`…) e mostra `1 — Ana`; escolher grava o id; FK composta (`tag_notes`: `a`, `b`) permanece como inputs simples.
- Abrir o registro 1 de `users` → seção "Referenciado por: posts · 2 registros (via user_id)"; clicar fecha o painel e abre `posts (user_id = 1)` com 2 linhas; o breadcrumb permite voltar.
- Registro de `users` sem dependentes não mostra a seção.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(frontend): FK combobox and referenced-by navigation"
```

---

### Task 14: Verificação final e build

**Files:**
- Modify: `.gitignore` (se necessário)
- Create: `README.md`

- [ ] **Step 1: Suíte completa**

Run: `go vet ./... && go test ./internal/... && (cd frontend && npm test && npm run build)`
Expected: tudo verde.

- [ ] **Step 2: Build de produção**

Run: `wails3 build`
Expected: gera `bin/sqliteviewer`.

- [ ] **Step 3: Roteiro manual contra os critérios de sucesso da spec**

Com `bin/sqliteviewer testdata/sample.db` (abertura por linha de comando):
1. O banco abre direto; fechar e rodar `bin/sqliteviewer` sem argumento reabre o último.
2. `bin/sqliteviewer /tmp/nao-existe.db` mostra erro e **não cria** o arquivo (`ls /tmp/nao-existe.db` falha).
3. Navegar `posts → user_id → users` e voltar; ver "Referenciado por" em `users`.
4. Inserir, editar e excluir; provocar NOT NULL, UNIQUE e FK e ver o erro no campo/mensagem certos.
5. Tabela `we"ird name` abre, edita e exclui normalmente; `tags` (WITHOUT ROWID, PK composta) edita e exclui; `log` (sem PK) edita por rowid; `blobs` mostra `<blob 4 bytes>` e o inteiro `9007199254740993` íntegro.
6. Criar um banco grande (`python3 -c "import sqlite3;c=sqlite3.connect('/tmp/big.db');c.execute('create table t(id integer primary key,v text)');c.executemany('insert into t(v) values (?)',[(str(i),) for i in range(500000)]);c.commit()"`) e confirmar que paginar, ordenar e filtrar continuam fluidos.

- [ ] **Step 4: README curto**

`README.md`: o que é, requisitos (Go, Node, `webkitgtk-6.0`, `wails3`), `wails3 dev`, `wails3 build`, uso (`sqliteviewer [arquivo.db]`), escopo do v1 e itens futuros (editor SQL, diagrama ER).

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "docs: README and final verification"
```

---

## Self-Review

**Cobertura da spec**

| Requisito da spec | Task |
|---|---|
| Listar tabelas/views, estrutura (colunas, PK, FKs saída/entrada) | 3, 12 (aba Estrutura) |
| Dados com paginação, ordenação e filtro no servidor | 4, 11 |
| FK clicável + "Referenciado por" | 6, 10 (`nav`), 11, 13 |
| Insert/update/delete em painel lateral, confirmação de delete | 5, 12 |
| Erros de constraint no campo certo | 5, 8 (`MarshalError`), 12 |
| Um banco por vez, recentes, reabrir o último, linha de comando | 2, 7, 8, 10 |
| Views somente leitura; rowid sem PK; WITHOUT ROWID | 3, 4, 5 |
| BLOB não editável; NULL distinto; "definir NULL" | 4, 12 |
| Estado vazio, tema claro/escuro | 10 (shadcn-vue traz o tema; o app segue o tema do sistema pelo `prefers-color-scheme`, ver nota abaixo) |
| Testes Go (FK composta, sem PK, WITHOUT ROWID) / Vitest (form e pilha de FK) | 2–8, 9–12 |

**Gap conhecido:** o toggle manual claro/escuro não tem tarefa própria. O shadcn-vue define as variáveis para `.dark`; falta aplicar a classe. Fica como ajuste pequeno dentro da Task 10 (`document.documentElement.classList.toggle('dark', matchMedia('(prefers-color-scheme: dark)').matches)` no `main.ts`), sem botão de alternância no v1.

**Consistência de tipos:** `NavEntry`, `RowQuery`, `Row.key`/`values`, `TableSchema` e os nomes dos métodos de `api` batem entre as Tasks 9–13 e com os nomes de método do `viewer.Service` (Task 8). `incomingTarget` aceita `RefCount` e `IncomingFK` porque só usa `table/from/to`.

**Riscos a confirmar durante a execução** (marcados nos passos): flags exatas de `wails3 init`/`generate bindings`; caminho de import dos bindings gerados; formato exato da mensagem de constraint do `modernc` (os testes da Task 5 acusam); versão do SQLite embutida para `pragma_table_list`.
