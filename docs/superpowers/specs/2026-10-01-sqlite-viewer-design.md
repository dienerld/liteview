# SQLite Viewer — Design (v1)

Data: 2026-10-01

## Objetivo

App desktop nativo e leve para visualizar e editar bancos SQLite, para uso pessoal (Linux). Não pretende competir com DBeaver: foco em simplicidade.

## Escopo do v1

**Dentro:**
- Listar tabelas e views; ver estrutura (colunas, PK, FKs de saída e de entrada).
- Navegar dados com paginação, ordenação e filtro por texto.
- Navegação por relacionamentos: célula de FK é link para a linha referenciada; seção "Referenciado por" mostra registros que apontam para a linha atual.
- Insert, update e delete por formulário em painel lateral.
- Um banco aberto por vez; lista de recentes; reabertura do último banco ao iniciar; abertura por linha de comando (`app meu.db`).

**Fora (fases futuras):**
- Editor de SQL livre.
- Diagrama ER visual (ex.: Vue Flow).
- Edição de BLOBs, edição inline na grid, múltiplos bancos simultâneos.

## Stack

- **Wails (Go)** como shell desktop (webview nativo).
- **Backend:** Go com `modernc.org/sqlite` (Go puro, sem CGO).
- **Frontend:** Vue 3 (`<script setup>`, TypeScript), Vite, Tailwind, shadcn-vue, `@tanstack/vue-table` (paginação no servidor), Pinia ou composables para estado.

## Abordagem de backend

API tipada em Go: o frontend chama métodos de alto nível (`ListTables`, `GetTableSchema`, `QueryRows`, `InsertRow`, `UpdateRow`, `DeleteRow` etc.) pelos bindings gerados do Wails. O SQL é montado no backend; nomes de tabela e coluna são validados contra o schema real e valores vão sempre como parâmetros. O frontend não conhece SQL.

Descartado: binding genérico `Exec(sql)` com SQL montado no Vue (joga segurança e lógica de PK para o frontend; só faria sentido com editor SQL).

## Backend (Go)

Pacotes pequenos, um propósito cada:

| Pacote | Responsabilidade |
|---|---|
| `db` | Abre/fecha conexão, ativa `PRAGMA foreign_keys=ON`, abre em modo leitura/escrita. |
| `schema` | Lê tabelas e views (`sqlite_master`), colunas (`PRAGMA table_info`), FKs de saída (`PRAGMA foreign_key_list`) e FKs de entrada (calculadas varrendo as FKs de todas as outras tabelas). |
| `rows` | Leitura paginada com ordenação e filtro simples; insert, update, delete. |
| `recents` | Lista de bancos recentes em JSON no diretório de config do usuário. |
| `app.go` | Casca fina do Wails: bindings, tratamento do argumento de linha de comando. |

**Identificação de linhas:** pela PK. Tabelas sem PK usam `rowid`. Tabelas `WITHOUT ROWID` sempre têm PK. Views são somente leitura.

**Erros:** violações de constraint (NOT NULL, UNIQUE, FK) retornam ao frontend com a coluna envolvida e mensagem legível, para o formulário marcar o campo.

**Testes:** testes de unidade em Go contra SQLite em memória, com schemas de exemplo incluindo FK composta, tabela sem PK e `WITHOUT ROWID`.

## Frontend (Vue + shadcn-vue)

**Layout:**
- **Sidebar esquerda:** nome do banco, botão de abrir e menu de recentes; lista de tabelas/views com busca (views com ícone distinto).
- **Área central:** abas "Dados" e "Estrutura" da tabela selecionada.
- **Painel lateral direito (Sheet):** formulário de insert/edição, aberto sob demanda.

**Aba Dados:**
- Grid com paginação, ordenação por coluna e filtro por texto nas colunas de texto, tudo resolvido no backend.
- Célula de FK é link: abre a tabela referenciada filtrada na linha de destino; pilha de navegação (breadcrumb) com botão de voltar.
- Clicar numa linha abre o painel em modo edição; botão "Novo" abre o mesmo painel em modo insert. Delete no painel e em menu da linha, sempre com diálogo de confirmação.

**Painel lateral (formulário):**
- Um campo por coluna, com input conforme o tipo declarado (texto, número, booleano, data, textarea para texto longo), marcação de obrigatório e default como placeholder.
- Campos de FK usam combobox com busca sobre as linhas da tabela referenciada.
- Seção "Referenciado por": por tabela de origem, quantidade de linhas que apontam para o registro, cada item levando à tabela filtrada.
- Erros de constraint do backend aparecem marcados no campo correspondente.

**Aba Estrutura:** colunas com tipo, NOT NULL, default e PK; FKs de saída e de entrada, cada uma clicável.

**Estados e detalhes:**
- Estado vazio sem banco aberto: botão de abrir e recentes.
- BLOBs exibidos como `<blob N bytes>`, não editáveis. NULL visualmente distinto de string vazia; o formulário tem controle explícito "definir NULL".
- Views ficam somente leitura, sem botões de insert/update/delete.
- Tema claro e escuro (shadcn-vue).

**Testes:** Vitest para componentes (validação e mapeamento de erros do formulário; pilha de navegação de FK), com bindings do Wails mockados.

## Critérios de sucesso

- Abrir um `.db` pelo seletor, pelos recentes ou pela linha de comando e ver suas tabelas.
- Navegar de uma FK à linha referenciada e voltar; ver quem referencia uma linha.
- Inserir, editar e excluir linhas com erros de constraint exibidos no campo certo.
- Tabelas grandes permanecem fluidas (paginação no servidor).
