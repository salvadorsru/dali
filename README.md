# Dalí

> Figma `.fig` knowledge engine — parse, index, search, and hand your design
> documents to AI agents.

[![CI](https://img.shields.io/github/actions/workflow/status/salvadorsru/dali/ci.yml?branch=master&label=CI&logo=github)](https://github.com/salvadorsru/dali/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/salvadorsru/dali?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/salvadorsru/dali?logo=github)](https://github.com/salvadorsru/dali/releases)
[![License](https://img.shields.io/github/license/salvadorsru/dali)](LICENSE)

Dalí reads a Figma `.fig` file and turns it into a queryable knowledge base. It
parses the document, indexes it in SQLite, and exposes the result to a CLI and
to AI agents over [MCP](https://modelcontextprotocol.io). The `.fig` file stays
the source of truth; everything else is a rebuildable local index.

```text
.fig ──► parser ──► model ──► index (SQLite + FTS5 + vec0)
                                   │
                                   ▼
                             query.Service
                             ┌──────┴──────┐
                           CLI            MCP
```

## Features

- Parses `.fig` and `.deck` archives (ZIP + Figma's Kiwi binary).
- Structural, full-text (FTS5) and semantic (vector) search, plus hybrid fusion.
- Incremental: only changed nodes are re-embedded.
- MCP server for AI agents, serving many designs from one process.
- Design linting and geometry analysis (spacing, clusters, overlaps).
- Component code generation: HTML + Tailwind plus design tokens.
- Image asset extraction.
- No CGO. SQLite and `sqlite-vec` are pure Go; embeddings use a managed
  `llama.cpp` binary downloaded on first use.

## Install

**Installer** — downloads the latest release and verifies its checksum:

```bash
curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/install.sh | sh
```

**Zero-install launcher** — caches the binary on first use, like `npx`:

```bash
curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/run.sh | sh -s -- design.fig inspect
```

**Go toolchain:**

```bash
go install github.com/salvadorsru/dali/cmd/dali@latest
```

Binaries for Linux, macOS and Windows (amd64 and arm64) are on the
[releases page](https://github.com/salvadorsru/dali/releases).

## Quick start

```bash
dali design.fig inspect
dali design.fig search "checkout"
dali design.fig semantic-search "payment screen"
dali design.fig generate --out ./components
dali design.fig mcp-config --all
```

The file is optional. When omitted, Dalí uses the single `.fig` in the current
directory.

The first semantic search downloads a pinned `llama.cpp` release and the
`ggml-org/embeddinggemma-300M` model (about 350 MB) into `~/.cache/dali`, after
showing the URLs, sizes and SHA256 hashes. Cancel with `n`, or skip the prompt
with `--yes` / `DALI_EMBED_YES=1`.

## CLI

```text
dali [<file.fig>] <command> [args]
```

The first argument can be a `.fig`/`.deck` file or a directory containing one.

Global flags: `--json` (machine-readable output), `--quiet`/`-q` (hide
progress), `--no-color`.

| Command | Description |
| --- | --- |
| `inspect` | Document counts (pages, nodes, components, assets, fonts, styles). |
| `search <query>` | Full-text search (FTS5). `--limit`, `--hybrid`. |
| `semantic-search <query>` | Vector search. `--limit`, `--type`, `--page`, `--hybrid`. |
| `node <id>` / `context <id>` | Show one node / its surroundings. |
| `children <id>` / `parent <id>` / `path <id>` / `subtree <id>` | Tree navigation; `subtree` prints a whole subtree with depth, styles and images (`--depth`). |
| `components` / `usages <id>` / `dependents <id>` | List components, where one is placed, and which screens depend on it. |
| `assets` / `fonts` / `styles` / `variables` | List image assets, fonts in use, style definitions, design tokens. |
| `query <expr>` | Structural query DSL (see grammar below). |
| `xpath <expr>` | XPath subset over the tree (`/`, `//`, `*`, `@name=`, `@name~`, `contains()`). `--limit`. |
| `lint [--preset P]` | Design + accessibility checks (`recommended`, `strict`, `accessibility`). |
| `analyze <kind>` | Geometry stats: `spacing`, `clusters`, `overlaps`. |
| `generate` | Generate HTML + Tailwind from components; `--out`, `--component`, `--tokens`. |
| `index [--embeddings] [--force]` | Build the structural index; `--embeddings` adds the vector index. |
| `extract-assets` | Write images to `<base>.assets/images`. |
| `mcp` | Run the MCP server over stdio. |
| `mcp-config [flags]` | Write MCP config files for AI editors. |
| `serve` / `stop` | Keep the managed embedding server warm / stop it. |
| `clean` | Stop the server and delete the downloaded engine + model. |
| `version` | Print version and build info. |

`--yes` is accepted by `search`, `index` and `semantic-search` to allow the model
download without prompting.

### Structural queries

`query` uses a small DSL:

| Expression | Meaning |
| --- | --- |
| `type=TEXT` | Exact node type (case-insensitive). |
| `name="Component"` | Exact node name. |
| `name~"button"` | Name substring (SQL `LIKE`). |
| `text~"checkout"` | Text content substring. |
| `font=Roboto` | Font family. |
| `page="Checkout"` | Page by name or id. |
| `component=1:2` | Instances of a component. |
| `visible=true` | Visibility. |
| `width=320` / `height>=100` | Numeric comparisons on `width`, `height`, `font_size` and `opacity` (`=`, `!=`, `>`, `>=`, `<`, `<=`). |
| `descendants("Component")` | Every descendant of the node named `Component`. |
| `ancestors("Component")` | Every ancestor of the node named `Component`. |
| `children("Component")` | Direct children of the node named `Component`. |
| `parent("Component")` | Parent of the node named `Component`. |

Terms combine with `AND`:

```bash
dali design.fig query 'type=TEXT'
dali design.fig query 'name~"button"'
dali design.fig query 'type=FRAME AND height>=100'
dali design.fig query 'descendants("Component")'
```

`xpath` evaluates a subset of XPath over the document tree, with type steps in
upper case (`/DOCUMENT/CANVAS`, `//FRAME`) and name predicates:

| Expression | Meaning |
| --- | --- |
| `/` | Child step. |
| `//` | Descendant step. |
| `*` | Any node type. |
| `@name="x"` | Exact name. |
| `@name~"x"` | Name substring. |
| `contains(@name,"x")` | Name substring. |

```bash
dali design.fig xpath '//FRAME[@name~"card"]'
dali design.fig xpath '/DOCUMENT/CANVAS/FRAME'
dali design.fig xpath '//*[contains(@name,"button")]'
```

XPath is the only read path evaluated in memory over a cached tree snapshot
(the DSL and the rest of `query` read SQL). It deliberately trades the
SQL pushdown for tree axes the DSL does not expose. A DSL query with no matches
prints name suggestions on stderr (`did you mean ...`).

### Output

Results print as aligned tables on **stdout**; progress and status messages go
to **stderr**. `--json` replaces the table with JSON on stdout, so it can be
piped safely:

```bash
dali design.fig search "checkout" --json | jq '.[0].path'
```

## Quality & analyze

`lint` and `analyze` always parse the `.fig` directly (they never rely on index
staleness).

`lint` reports design-quality and accessibility issues. Presets:
`recommended` (default), `strict` (promotes warnings to errors), and
`accessibility` (accessibility rules only, all reported as errors):

```text
RULE             SEVERITY  NODE        NAME        PATH               DETAIL
text-contrast    warn      0:3         Body        Doc/Page/Card/Body contrast 1.00:1 ...
generic-name     warn      0:4         Frame 12    ...                auto-generated name
touch-target     warn      ...         ...         ...                min side 10px below 44px
unused-component warn      0:8         IconBtn     ...                component has no instances
```

`analyze` inspects geometry:

```bash
dali design.fig analyze spacing    # most-used layout gaps
dali design.fig analyze clusters   # repeated node sizes
dali design.fig analyze overlaps   # intersecting siblings (--min-ratio 0.1)
```

Vector geometry, fills, corner radii, clipping, text and embedded images are
included; raster formats are out of scope.

## Component code generation

`generate` translates components and component sets into HTML + Tailwind:

```bash
dali design.fig generate                              # all components
dali design.fig generate --component 1:123 --out ./components
dali design.fig generate --tokens css
```

Each component is written as a self-contained HTML file under `--out` (default
`./generated`); text content is escaped, so strings like `-15%` stay valid.
`--tokens` also exports design tokens as CSS variables (`css`), Style Dictionary
JSON (`style-dict`) or Figma Tokens JSON (`figma`).

## Semantic search

Embeddings come from an `embed.Embedder`. The default provider (`auto`)
manages an official `llama.cpp` `llama-server`, downloading it and the model on
first use. If its tuned server flags fail, Dalí retries with a minimal
configuration.

| `DALI_EMBED_PROVIDER` | Behavior |
| --- | --- |
| `auto` (default) | Use `DALI_LLAMA_URL`, else `DALI_EMBED_URL`, else managed llama.cpp. |
| `llama` | Managed llama.cpp (or `DALI_LLAMA_URL` if set). |
| `ollama` | Ollama at `DALI_EMBED_URL`. |
| `remote` | Any OpenAI-compatible `/v1/embeddings` at `DALI_EMBED_URL`. |

Key variables: `DALI_CACHE_DIR` (default `~/.cache/dali`), `DALI_LLAMA_URL`
(reuse a running server), `DALI_LLAMA_MODEL`, `DALI_LLAMA_TAG` (pinned release),
`DALI_LLAMA_PARALLEL` and `DALI_LLAMA_THREADS` (managed server slots/threads),
`DALI_LLAMA_KEEP` (`0` stops the server after each command), `DALI_EMBED_URL`,
`DALI_EMBED_MODEL`, `DALI_EMBED_BATCH` and `DALI_EMBED_CONCURRENCY`. Full
reference in [AGENTS.md](AGENTS.md).

### Incremental indexing

```bash
dali index                     # structural only
dali index --embeddings        # structural + vector
dali index --force             # rebuild even if up to date
```

- Only nodes whose semantic text changed are re-embedded.
- Nodes with auto-generated names and no text (for example `Vector` or
  `Frame 12`) are not embedded; they remain structurally searchable.
- Changing the model or bumping `semantic.GeneratorVersion` drops and rebuilds
  the vector index; everything else is incremental.
- The CLI builds embeddings on an **explicit** command only (`index
  --embeddings`, `semantic-search`, `search --hybrid`); a search never index on
  its own.

The managed server is left running after a command, so the next search reuses
it: `dali serve` keeps it alive between sessions, `dali stop` stops it, and
`dali clean` also frees the ~350 MB cache (vectors stay in SQLite).

## MCP

Run the server over stdio with `dali design.fig mcp`. One server serves many
designs; every tool accepts an optional `file` argument, or falls back to the
fig passed to `dali mcp` and then to the single `.fig` in the working directory.

| Tool | Description |
| --- | --- |
| `search_figma` | Search by name/text/meaning. `mode`: `text` (default, exact keyword + near-miss suggestions), `semantic` (conceptual, needs the vector index), `hybrid` (fuses text+vectors, falls back to full-text). `type` and `page` filters. Never builds embeddings. |
| `section_figma` | Resolve a named block and inspect it in one call. Prefer for exact names. |
| `index_embeddings` | Build the vector index explicitly. The only tool that embeds; searches never do. |
| `query_figma` | Structural expressions, e.g. `type=FRAME`, `name~"card"`, `children("Header")`, `width>=320`. |
| `xpath_figma` | XPath subset over the tree, e.g. `//FRAME[@name~"card"]`, `/DOCUMENT/CANVAS`. |
| `get_node` / `get_context` / `get_children` | Node inspection. |
| `get_descendants` | Whole subtree with depth, styles and images. |
| `get_parent` | Parent node. |
| `find_components` / `find_instances` | List components / instances, optionally by name (empty results suggest nearby names). |
| `find_assets` / `get_image` / `extract_assets` | List assets / fetch one as base64 / write them to `<base>.assets/images`. |
| `get_fonts` / `get_styles` | List fonts / styles, optionally filtered by name or kind. |
| `get_tokens` | Aggregate tokens: colour palette, variables, fonts, text classes, layouts. |
| `get_usages` / `get_component` | Component placements / codegen contract (variants, props). |
| `get_dependents` | Impact analysis: which screens place a component. |
| `variables` | List design tokens with values per mode. |
| `get_stats` / `list_pages` | Document summary / pages in document order. |
| `list_figs` | List `.fig`/`.deck` files in the current directory. |
| `lint_figma` | Design + accessibility checks (presets: `recommended`, `strict`, `accessibility`). |
| `analyze_figma` | Geometry analysis: `spacing`, `clusters`, `overlaps`. |

### Configuration

Generate the config file for your editor with absolute paths:

```bash
dali mcp-config                                  # all project-level editors
dali design.fig mcp-config --all                 # ...and Windsurf (global)
dali mcp-config --print --vscode                 # print instead of writing
```

Supported editors: OpenCode (`opencode.json`), Cursor (`.cursor/mcp.json`),
Claude Code (`.mcp.json`), VS Code (`.vscode/mcp.json`), Cline (`.cline/mcp.json`),
Roo Code (`.roo/mcp.json`), and Windsurf (global). Existing files are **merged**,
not replaced; the design path is written absolute.

### Embeddings are opt-in

The server **never** starts an embedder unless `DALI_EMBEDDINGS=1` is set. Search
tools stay offline by default; `search_figma` in `hybrid` mode degrades to
full-text and `semantic` mode fails with a hint. To enable vectors, add to the
server `env` (in the config file) or pass on the command line:

```json
"env": {
  "DALI_EMBEDDINGS": "1",
  "DALI_EMBED_PROVIDER": "auto",
  "DALI_EMBED_YES": "1"
}
```

`DALI_EMBEDDINGS=1` is the opt-in gate. `DALI_EMBED_PROVIDER` picks the provider
and `DALI_EMBED_YES` skips the download confirmation (there is no terminal over
stdio). `mcp-config` does not write any of these by default.

Then build the vectors explicitly — searches never index:

```bash
dali design.fig index --embeddings
# or through the MCP server:
#   index_embeddings
```

OpenCode's `timeout` defaults to 5000 ms, and first start can exceed it, so the
generated config raises it to 120000 ms.

When a tool opens a stale index, the server rebuilds it on the fly, reporting
progress through `notifications/progress` when the client sends a
`_meta.progressToken` and via `dali: …` lines on stderr.

## Generated files

For `design.fig`:

```text
design.fig
design.assets/              # extracted assets (user-facing)
    images/
.dali/                      # technical files
    design.fig.db           # SQLite index (rebuildable)
```

`.dali/` is disposable: delete it and Dalí rebuilds the index from the `.fig`.

## Architecture

```text
parser.Open ─► kiwi decode ─► model.Build ─► index.Write ─► query.Service ─► CLI / MCP
                                                                   │
                                                        embed ─► semantic.Index
                                                        (vec0 vector index)
```

| Path | Responsibility |
| --- | --- |
| `kiwi/` | Kiwi binary format: schema decode and dynamic message decode. |
| `parser/` | ZIP and chunk decompression into a raw message. |
| `model/` | Node tree built from the flat `nodeChanges` array. |
| `index/` | SQLite schema, writes, FTS5, and the `vec_nodes` vector table. |
| `query/` | All reads; owns SQL. |
| `embed/` | `Embedder` interface, OpenAI-compatible client, managed llama.cpp. |
| `semantic/` | Semantic text per node and the incremental embedding index. |
| `quality/` | Design linting and geometry analysis (spacing, clusters, overlaps). |
| `codegen/` | HTML + Tailwind code generation and token export. |
| `assets/` | Image extraction. |
| `workspace/` | Path layout for the technical `.dali/` directory. |
| `output/` | Table and JSON output. |
| `mcp/` | MCP server. |
| `cmd/dali/` | CLI. |

## Development

```bash
go build -o dali ./cmd/dali
go test ./...
go vet ./...
```

FTS5 and `sqlite-vec` are compiled into `modernc.org/sqlite` — no build tags
needed. Integration tests and the parser benchmark use `../woments.fig` and skip
when it is absent. Live tests are opt-in: `DALI_NET_TEST=1` and
`DALI_MANAGED_TEST=1`.

## Contributing

- Keep the layer separation: `parser` → `model` → `index` → `query`. All read
  SQL lives in `query/`.
- Use the fake embedder for anything that touches embeddings.
- Run `go test ./...` and `go vet ./...` before opening a PR.
- See [AGENTS.md](AGENTS.md) for architecture details and conventions.

## License

MIT. See [LICENSE](LICENSE).
