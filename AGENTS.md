# AGENTS.md

## Build & Test

```bash
go build -o dali ./cmd/dali
go test  ./...
go vet   ./...
```

FTS5 comes compiled into `modernc.org/sqlite` (pinned v1.59.0); no build tag is
needed. Older versions required `-tags sqlite_fts5`, so do not downgrade the
dependency without re-checking.

Integration tests and the parser benchmark use `../woments.fig` and skip when absent.

## Architecture

`.fig` is the source of truth. SQLite (`<file>.fig.db`) is a rebuildable index.
`query.Service` is the core; CLI and MCP are consumers.

```
parser.Open  -> kiwi decode -> model.Build -> index.Write -> query.Service -> CLI / MCP
                                                                  |
                                                       embed -> semantic.Index
                                                       (vec0 vector index)
```

- `kiwi/` — Kiwi binary format: schema decode + dynamic message decode into `map[string]any`.
- `parser/` — ZIP + chunk decompression (raw-deflate or zstd) → raw `Message`.
- `model/` — tree built from the flat `nodeChanges` array via `parentIndex`.
- `index/` — SQLite schema, write, FTS5, and the `vec_nodes` vector table.
- `query/` — all reads; owns SQL. CLI/MCP never touch SQL.
- `embed/` — `Embedder` interface + Ollama client (`/api/embed`).
- `semantic/` — semantic text per node + incremental embedding index.
- `assets/` — extract images to `<base>.assets/images` (next to the .fig).
- `workspace/` — path layout for the technical `.dali/` directory.
- `output/`, `mcp/`, `cmd/dali/`.

MCP tools accept an optional `file` argument, so one server can serve several
figs; `list_figs` discovers them. Lazy indexing is reported via MCP progress
notifications (when the client sends `_meta.progressToken`) and always via
`dali:` lines on stderr. `dali mcp-config` writes each editor's config
(`--opencode|--cursor|--claude|--vscode|--cline|--roo|--windsurf`, or `--all`)
into the current directory, merging with existing files: OpenCode `opencode.json`
(`mcp`), Cursor `.cursor/mcp.json`, Claude Code `.mcp.json`, VS Code
`.vscode/mcp.json` (`servers`), Cline `.cline/mcp.json`, Roo Code `.roo/mcp.json`,
and Windsurf's global `~/.codeium/windsurf/mcp_config.json`.

## Semantic search

- sqlite-vec ships inside `modernc.org/sqlite/vec` (CGo-free, v0.1.9) and is
  auto-registered by blank import. **No extra build tag, no driver change.**
- `vec_nodes` uses `node_id TEXT PRIMARY KEY` + `embedding float[N] distance_metric=cosine`.
- Embeddings come from an `embed.Embedder`. Default provider (`DALI_EMBED_PROVIDER=auto`)
  manages an official **llama.cpp** `llama-server` and the **ggml-org/embeddinggemma-300M-GGUF**
  model: it downloads them into `DALI_CACHE_DIR` (default `~/.cache/dali`) after
  confirmation, verifies SHA256, starts the server, and stops it when done.
- Providers: `auto` | `ollama` | `llama` | `remote` (`DALI_EMBED_PROVIDER`).
  `DALI_LLAMA_URL` reuses an already-running llama-server. `DALI_EMBED_URL`
  points at Ollama. `DALI_EMBED_YES=1` / `--yes` skips the download prompt.
- The managed server is recorded in `$DALI_CACHE_DIR/server.json` and left
  running so later commands reuse it (`dali serve`, `dali stop`,
  `DALI_LLAMA_KEEP=0` to stop after each command).
- Performance: embedding runs through a worker pool (`embed.Concurrency()`) that
  feeds a single SQLite writer; llama-server `--parallel`/`-t`/`-tb` are derived
  from the CPU count, with a conservative fallback if the tuned flags fail.
  Tune with `DALI_LLAMA_PARALLEL`, `DALI_LLAMA_THREADS`, `DALI_EMBED_CONCURRENCY`,
  `DALI_EMBED_BATCH`.
- Nodes with an auto-generated name and no text (e.g. `Vector`, `Frame 12`) are
  not embedded; they still exist for structural search.
- Changing model or `semantic.GeneratorVersion` drops and rebuilds the vector index.
- `dali semantic-search` auto-indexes missing/changed embeddings before searching.
- Tests use a deterministic fake embedder; live tests are opt-in
  (`DALI_NET_TEST=1`, `DALI_MANAGED_TEST=1`).

## Generated files (gitignored)

Technical files go under `.dali/` next to the fig (SQLite index: `.dali/<file>.db`).
User-facing output stays next to the fig: `<base>.assets/` (and future JSON exports).
