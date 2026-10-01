package viewer

import (
	"database/sql"
	"errors"
	"sync"

	appdb "liteview/internal/db"
	"liteview/internal/recents"
	"liteview/internal/rows"
	"liteview/internal/schema"
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
