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
