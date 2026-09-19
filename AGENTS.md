# AGENTS.md

## Build & Test

Always pass `-tags sqlite_fts5`; the FTS5 virtual table fails at runtime without it.

```bash
go build -tags sqlite_fts5 -o dali ./cmd/dali
go test  -tags sqlite_fts5 ./...
go vet   -tags sqlite_fts5 ./...
```

Integration tests and the parser benchmark use `../woments.fig` and skip when absent.

## Architecture

`.fig` is the source of truth. SQLite (`<file>.fig.db`) is a rebuildable index.
`query.Service` is the core; CLI and MCP are consumers.

```
parser.Open  -> kiwi decode -> model.Build -> index.Write -> query.Service -> CLI / MCP
```

- `kiwi/` — Kiwi binary format: schema decode + dynamic message decode into `map[string]any`.
- `parser/` — ZIP + chunk decompression (raw-deflate or zstd) → raw `Message`.
- `model/` — tree built from the flat `nodeChanges` array via `parentIndex`.
- `index/` — SQLite schema, write, FTS5.
- `query/` — all reads; owns SQL. CLI/MCP never touch SQL.
- `assets/` — extract images to `<base>.assets/images`.
- `output/`, `mcp/`, `cmd/dali/`.

## Generated files (gitignored)

`*.fig.db`, `*.assets/`.
