# Dalí

**Local search and indexing engine for Figma `.fig` files.**

Dalí parses Figma `.fig` files on disk, builds a rebuildable local SQLite index
(full-text, structural and vector), and exposes search through a CLI and an MCP
server. Figma does not need to be open, and designs stay local.

The `.fig` file remains the source of truth. Everything Dalí generates is a
rebuildable local index under `.dali/`.

## Features

- **Local `.fig` parsing** — decode Figma's Kiwi binary format directly, no Figma
  running, so any design file is searchable offline.
- **SQLite index** — persistent, rebuildable index stored under `.dali/`.
- **Structural search** — query the document tree and node relationships
  (`query_figma` DSL, XPath subset via a leading `/`).
- **Full-text search** — SQLite FTS5 over node names and text content.
- **Semantic search** — conceptual vector search via `sqlite-vec`.
- **Local embeddings** — optional automatic embeddings with `llama.cpp` +
  EmbeddingGemma (`mode=auto`), kept running for reuse.
- **Ollama & remote embeddings** — use an existing Ollama server or a remote
  provider instead.
- **Incremental indexing** — only missing or changed embeddings are embedded.
- **Design linting** — accessibility and design-quality checks with presets.
- **Geometry analysis** — layout gaps, repeated sizes, sibling overlaps.
- **Component code generation** — HTML + Tailwind (or CSS variables) plus
  design tokens (CSS variables, Style Dictionary JSON, Figma Tokens JSON).
- **Name suggestions** — near-miss hints when a search or filter matches nothing.
- **MCP server** — expose one or many `.fig` files to MCP clients.
- **Multi-client support** — config helpers for OpenCode, Cursor, Claude Code,
  VS Code, Cline, Roo Code, Windsurf.
- **Asset extraction** — images written next to the original `.fig`.
- **CGo-free** — SQLite and `sqlite-vec` via `modernc.org/sqlite`.

## Architecture

```text
            .fig
             │
         parser.Open            ── kiwi decode
             │
        model.Build             ── document tree
             │
      ┌──────┴───────┐
      ▼              ▼
   SQLite          Semantic
   index           indexing
   FTS5               │
   query/             ▼
             embeddings (sqlite-vec)
      │              │
      └──────┬───────┘
             ▼
      query.Service
        │        │
       CLI      MCP
```

The same `query.Service` powers the CLI and the MCP server.

Core pipeline: `kiwi/` → `parser/` → `model/` → `index/` → `query/`, plus
`embed/`, `semantic/`, `mcp/` and `cmd/dali/`.

## Quickstart

```bash
# build
go build -o dali ./cmd/dali

# index a design
./dali design.fig index

# structural search
./dali design.fig query 'type=FRAME name~"card"'
./dali design.fig xpath '//FRAME[@name~"card"]'

# full-text
./dali design.fig search "payment"

# semantic (builds the vector index first)
./dali design.fig semantic "checkout flow" --embeddings --yes

# lint + geometry
./dali design.fig lint --preset accessibility
./dali design.fig analyze --mode overlaps

# codegen
./dali design.fig generate
./dali design.fig generate --component 1:123 --out ./components
./dali design.fig generate --tokens css
```

## MCP

Mount one or many `.fig` files and let MCP clients query them through Dalí's
tools. Generate per-client configs with:

```bash
dali mcp-config --opencode        # also: cursor, claude, vscode, cline, roo, windsurf, all
```

`--all` writes configs for every supported editor; existing config files are
merged, never clobbered.

| Client      | Configuration                         |
| ----------- | ------------------------------------- |
| OpenCode    | `opencode.json`                       |
| Cursor      | `.cursor/mcp.json`                    |
| Claude Code | `.mcp.json`                           |
| VS Code     | `.vscode/mcp.json`                    |
| Cline       | `.cline/mcp.json`                     |
| Roo Code    | `.roo/mcp.json`                       |
| Windsurf    | `~/.codeium/windsurf/mcp_config.json` |

### Search tools

Structural/full-text/semantic tools are exposed to MCP clients:

```
analyze_figma    design-quality checks (spacing, clusters, overlaps)
find_assets      list image assets
find_components  list components
find_instances   list component instances
generate_figma   HTML + Tailwind codegen for components and tokens
get_assets       list image assets
get_context      structural context for a node
get_descendants  fetch a whole subtree in one call
get_dependents   impact analysis of component placement
get_styles       list style definitions
get_tokens       aggregate design tokens (incl. fonts, variables)
get_usages       list page placements of a component
index_embeddings build the vector index (the only embedding builder)
list_pages       list document pages
lint_figma       design-quality and accessibility linting
query_figma      structural DSL query over the document tree (leading "/" = XPath)
search_figma     find nodes: mode = text | semantic | hybrid
section_figma    resolve a named section/component with its subtree
```

### Search contracts

- **`search_figma`** is the only search entry point. Arguments: `mode`,
  `query`, `type`, `page`, `limit` (default 20), plus structural filters.
  Search **never** builds embeddings.
- `index_embeddings` is the **only** tool that builds embeddings. Searches
  never do — call `index_embeddings` first when semantic/hybrid search fails
  with a hint.
- `query_figma` operates structurally and never uses embeddings; a leading `/`
  routes to the XPath subset.
- Instances are found via `find_instances`; assets via `find_assets`.

| Tool | Embeddings | Indexes |
| --- | --- | --- |
| `search_figma` (mode=text, `query_figma`, ...) | never | never |
| `search_figma` (mode=semantic/hybrid) | only if vector index exists + `DALI_EMBEDDINGS=1` | never |
| `index_embeddings` | yes — the only tool that does | yes |

## Semantic search

`mode=auto` manages an official `llama.cpp` `llama-server` bundling
`ggml-org/embeddinggemma-300M-GGUF`, downloaded into `~/.cache/dali`. The server
stays alive so later commands reuse it.

Supported providers: `auto`, `ollama`, `llama`, `remote`. Set
`DALI_EMBED_PROVIDER=off` to disable.

### Configuration

| Variable                 | Purpose                                    |
| ------------------------ | ------------------------------------------ |
| `DALI_EMBED_PROVIDER`    | `auto`, `ollama`, `llama`, `remote`, or `off` |
| `DALI_EMBEDDINGS=1`      | opt-in: enable embeddings in the MCP server |
| `DALI_CACHE_DIR`         | Dalí cache directory                       |
| `DALI_LLAMA_URL`         | reuse an existing `llama-server`           |
| `DALI_EMBED_URL`         | Ollama embedding endpoint                  |
| `DALI_EMBED_YES=1`       | skip download confirmation                 |
| `DALI_LLAMA_KEEP=0`      | stop the managed server after each command |
| `DALI_LLAMA_PARALLEL`    | override llama-server parallelism          |
| `DALI_LLAMA_THREADS`     | override llama-server threads              |
| `DALI_EMBED_CONCURRENCY` | embedding worker concurrency               |
| `DALI_EMBED_BATCH`       | embedding batch size                       |
| `DALI_NET_TEST=1`        | enable live network tests                  |
| `DALI_MANAGED_TEST=1`    | enable managed-server tests                |

Embedding generation uses a worker pool and a single SQLite writer. If the
model or `semantic.GeneratorVersion` changes, the vector index is rebuilt.

## Files

Dalí never modifies the original `.fig`.

```text
design.fig        original (untouched)
.dali/design.db   rebuildable SQLite index (deletable)
design.assets/
  images/         extracted user-facing assets
```

Deleting `.dali/` removes generated index data only; the design stays intact.

## Development

Requirements: Go.

```bash
go build -o dali ./cmd/dali
go test ./...
go vet ./...
```

Integration tests and the parser benchmark read `../dali.fig` and are skipped
automatically when absent.

Note: never downgrade `modernc.org/sqlite` — FTS5 is compiled into it.
`sqlite-vec` is provided via `modernc.org/sqlite/vec` and registers itself
through a blank import.

## License

```
Unlicense — public domain.
```
