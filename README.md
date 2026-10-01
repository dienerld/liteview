# Liteview

App desktop simples para visualizar e editar bancos SQLite, feito com Wails v3 (Go) + Vue 3 + shadcn-vue.

## O que faz (v1)

- Lista tabelas e views; mostra a estrutura (colunas, PK, chaves estrangeiras de saída e de entrada).
- Navega pelos dados com paginação, ordenação e filtro de texto (tudo no backend).
- Segue relacionamentos: valores de FK são links, e cada registro mostra "Referenciado por" com a contagem de registros que apontam para ele.
- Insere, edita e exclui registros por um formulário em painel lateral, com erros de constraint exibidos no campo certo.
- Um banco por vez, com lista de recentes, reabertura do último banco e abertura por linha de comando.

Fora do v1: editor SQL, diagrama ER, edição inline, múltiplos bancos, edição de BLOB.

## Requisitos

- Go ≥ 1.24, Node.js + npm
- CLI do Wails v3: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26`
- Linux: `gcc`, `gtk4` e `webkitgtk-6.0` (confira com `wails3 doctor`)

## Uso

```
wails3 dev                     # desenvolvimento (hot reload)
wails3 build                   # gera bin/liteview
bin/liteview [arquivo.db]      # abre direto um banco
```

Banco de exemplo para testes manuais:

```
python3 -c "import sqlite3; sqlite3.connect('testdata/sample.db').executescript(open('testdata/seed.sql').read())"
```

## Testes

```
go test ./internal/...
cd frontend && npm test
```

## Estrutura

- `internal/db`, `internal/schema`, `internal/rows`, `internal/recents`: backend em Go, sem dependência do Wails.
- `internal/viewer`: o único service do Wails; seus métodos públicos viram os bindings do frontend.
- `frontend/`: Vue 3 + TypeScript; `src/lib/api.ts` é o único arquivo que importa os bindings gerados.
- `docs/superpowers/`: spec e plano de implementação.

## Licença

[MIT](LICENSE)
