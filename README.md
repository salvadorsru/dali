# Dalí

> Figma `.fig` knowledge engine — parse, index, search, and hand your design
> documents to AI agents.

[![CI](https://img.shields.io/github/actions/workflow/status/salvadorsru/dali/ci.yml?branch=master&label=CI&logo=github)](https://github.com/salvadorsru/dali/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/salvadorsru/dali?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/salvadorsru/dali?logo=github)](https://github.com/salvadorsru/dali/releases)
[![License](https://img.shields.io/github/license/salvadorsru/dali)](LICENSE)

Dalí reads a Figma `.fig` file and turns it into a queryable knowledge base. It
parses the document, indexes it in SQLite, and exposes the result to a CLI and
to AI agents over [MCP](https://modelcontextprotocol.io).

```text
.fig ──► parser ──► model ──► index (SQLite + FTS5 + vec0)
                                   │
                                   ▼
                             query.Service
                             ┌──────┴──────┐
                           CLI            MCP
```

`.fig` is the source of truth. SQLite is a rebuildable index. `query.Service`
is the core; the CLI and the MCP server are consumers of the same engine.

## Features

- Reads `.fig` and `.deck` archives (ZIP + Figma's Kiwi binary) with a
  self-describing Kiwi decoder.
- Rebuilds the node tree from the flat `nodeChanges` array.
- Indexes nodes, pages, components, instances, assets, styles and fonts.
- Full-text search (FTS5) over node names and text.
- Semantic (vector) search with `sqlite-vec` (`vec0`) and embeddings, plus
  hybrid full-text + vector fusion.
- Structural queries: `type=`, `name~`, `descendants()`, `ancestors()`, ...
- Extracts image assets to disk, in parallel.
- MCP server for AI agents, serving many designs from one process.
- Incremental: unchanged nodes are never re-embedded.
- No CGO. SQLite and `sqlite-vec` are pure Go; the embedding engine is a
  managed `llama.cpp` binary downloaded on first use.

## Install

Prebuilt binaries for Linux, macOS and Windows (amd64 and arm64), plus SHA256
checksums, are published on the
[releases page](https://github.com/salvadorsru/dali/releases).

**Installer** — downloads the latest release and verifies its checksum:

```bash
curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/install.sh | sh
```

The installer puts `dali` in `~/.local/bin` and prints a PATH hint if needed.

**Zero-install launcher** — caches the binary on first use and reuses it, like
`npx` or `uvx`:

```bash
curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/run.sh | sh -s -- design.fig inspect
```

**Go toolchain:**

```bash
go install github.com/salvadorsru/dali/cmd/dali@latest
```

### Install environments

| Variable | Used by | Default | Description |
| --- | --- | --- | --- |
| `DALI_REPO` | both scripts | `salvadorsru/dali` | GitHub repository to fetch from. |
| `DALI_VERSION` | both scripts | `latest` | Release tag to install. |
| `DALI_BIN` | `install.sh` | `~/.local/bin` | Install directory. |
| `DALI_CACHE_DIR` | `run.sh` | `~/.cache/dali` | Where the launcher caches the binary. |

> Windows note: `install.sh` targets Linux and macOS. Windows users can grab
> the `dali_windows_amd64.zip` / `dali_windows_arm64.zip` asset from the
> releases page, or use the launcher under Git Bash.

## Quick start

```bash
dali design.fig inspect
dali design.fig search "checkout"
dali design.fig semantic-search "payment screen"
dali design.fig mcp-config --all
```

The file is optional. When omitted, Dalí uses the single `.fig` in the current
directory, and a bare `dali` greets you with what it found:

```bash
cd ~/designs/my-project
dali                          # Found design.fig. Try: ...
dali inspect
dali semantic-search "payment screen"
```

The first semantic search downloads the pinned `llama.cpp` release and the
`ggml-org/embeddinggemma-300M` model (about 350 MB) into `~/.cache/dali`, after
showing the URLs, sizes and SHA256 hashes. Cancel with `n`, or pre-approve with
`--yes` / `DALI_EMBED_YES=1`.

Node ids use the `sessionID:localID` form, for example `123:456`.

## CLI reference

```text
dali [<file.fig>] <command> [args]
```

The first argument can be a `.fig`/`.deck` file or a directory containing one;
everything after it is the command. `-v` / `--version` and `help` work too.

### Global flags

| Flag | Description |
| --- | --- |
| `--json` | Machine-readable JSON output on stdout. |
| `--no-color` | Disable ANSI colors. `NO_COLOR=1` has the same effect. |
| `--quiet`, `-q` | Hide progress bars and status spinners. |

`--yes` is accepted by `index` and `semantic-search` to allow the model
download without prompting.

### Commands

| Command | Description |
| --- | --- |
| `inspect` | Document counts (pages, nodes, components, assets, fonts, styles). |
| `search <query>` | Full-text search (FTS5). |
| `semantic-search <query> [flags]` | Vector search, with optional filters. |
| `node <id>` | Show one node. |
| `context <id>` | Path, parent, children, component, styles, assets. |
| `children <id>` | Direct children. |
| `parent <id>` | Parent node. |
| `path <id>` | Root-to-node path. |
| `components` | List components. |
| `assets` | List image assets. |
| `fonts` | List fonts in use. |
| `query <expr>` | Structural query (see grammar below). |
| `index [flags]` | Build the structural index; `--embeddings` adds the vector index. |
| `extract-assets` | Write images to `<base>.assets/images`. |
| `mcp` | Run the MCP server over stdio. |
| `mcp-config [flags]` | Write MCP config files for AI editors. |
| `serve` | Keep the managed embedding server warm (reused later). |
| `stop` | Stop the managed embedding server. |
| `version` | Print version and build info. |

### Command flags

`semantic-search`:

| Flag | Description |
| --- | --- |
| `--limit N` | Max results (default 20). |
| `--type T` | Filter by node type (`FRAME`, `TEXT`, `SYMBOL`, ...). |
| `--page P` | Filter by page name or id. |
| `--hybrid` | Fuse full-text and vector results with Reciprocal Rank Fusion. |
| `--yes` | Allow the model download without prompting. |

`index`:

| Flag | Description |
| --- | --- |
| `--embeddings` | Also build the semantic (vector) index. |
| `--force` | Rebuild even if the index is up to date. |
| `--yes` | Allow the model download without prompting. |

`mcp-config`:

| Flag | Description |
| --- | --- |
| `--opencode` | Write `opencode.json`. |
| `--cursor` | Write `.cursor/mcp.json`. |
| `--claude` | Write `.mcp.json`. |
| `--vscode` | Write `.vscode/mcp.json`. |
| `--cline` | Write `.cline/mcp.json`. |
| `--roo` | Write `.roo/mcp.json`. |
| `--windsurf` | Write the global `~/.codeium/windsurf/mcp_config.json`. |
| `--all` | Write a config for every editor, including Windsurf. |
| `--src FILE` | Point at a design file; defaults to the fig in the current directory. |
| `--dir DIR` | Write the project configs into `DIR` instead of the current directory. |
| `--print` | Print the configs to stdout instead of writing files. |

Without an editor flag, every project-level config is written (all of the above
except `--windsurf`).

### Structural query grammar

`dali query '<expr>'` filters or walks the tree:

| Expression | Meaning |
| --- | --- |
| `type=TEXT` | Exact node type (values are compared case-insensitively). |
| `name="Checkout"` | Exact node name. |
| `name~"button"` | Name substring (SQL `LIKE`). |
| `descendants("Checkout")` | Every descendant of the node **named** `Checkout`. |
| `ancestors("Checkout")` | Every ancestor of the node named `Checkout`. |
| `children("Checkout")` | Direct children of the node named `Checkout`. |
| `parent("Checkout")` | Parent of the node named `Checkout`. |

Values may be quoted with `'` or `"`. Relation functions resolve their argument
as a node *name*, so they need a unique, recognizable label.

```bash
dali design.fig query 'type=TEXT'
dali design.fig query 'name~"button"'
dali design.fig query 'descendants("Checkout")'
```

### Output format

Results print as aligned, color-coded tables on **stdout**; progress bars,
spinners and status messages go to **stderr**. `--json` replaces the table with
JSON on stdout, so it can be piped safely:

```bash
dali design.fig search "checkout" --json | jq '.[0].path'
```

Colors and spinners appear only when stdout/stderr are a terminal and `NO_COLOR`
is unset; otherwise output falls back to plain text.

### Example output

```text
$ dali design.fig inspect
Actions                             # ← styled title
  pages:      8
  nodes:      4 231
  components: 126
  instances:  1 402
  images:     89
  fonts:      17
  styles:     164

Node types
  TYPE   COUNT
  FRAME  1 802
  TEXT   1 156
  ...
```

```text
$ dali design.fig search "checkout"
ID          TYPE   NAME           PATH
123:4567    TEXT   Checkout        UI Desktop / Cart / Header
...
```

```text
$ dali design.fig semantic-search "primary checkout button" --limit 5
ID          TYPE   SCORE  NAME                 PATH
123:8201    SYMBOL 0.847  Button / Primary     UI Desktop / Cart / Actions
...
```

## Semantic search

Embeddings come from an `embed.Embedder`. The default provider
(`DALI_EMBED_PROVIDER=auto`) manages an official `llama.cpp` server:

1. Resolve the pinned `llama.cpp` release for your OS and architecture.
2. Download it and verify its SHA256.
3. Download and verify the GGUF model.
4. Start `llama-server`, embed, and reuse or stop it.

If the tuned server flags fail on a machine, Dalí retries with a minimal
configuration, so it also works on small laptops and unusual builds.

### Providers

| `DALI_EMBED_PROVIDER` | Behavior |
| --- | --- |
| `auto` (default) | Use `DALI_LLAMA_URL`, else `DALI_EMBED_URL`, else managed llama.cpp. |
| `llama` | Managed llama.cpp (or `DALI_LLAMA_URL` if set). |
| `ollama` | Ollama at `DALI_EMBED_URL`. |
| `remote` | Any OpenAI-compatible `/v1/embeddings` at `DALI_EMBED_URL`. |

### Environment

| Variable | Default | Description |
| --- | --- | --- |
| `DALI_EMBED_PROVIDER` | `auto` | `auto`, `llama`, `ollama` or `remote`. |
| `DALI_LLAMA_URL` | | Use an already-running `llama-server`. |
| `DALI_LLAMA_MODEL` | `ggml-org/embeddinggemma-300M-GGUF` | Managed model repo. |
| `DALI_LLAMA_FILE` | | Managed model file (optional). |
| `DALI_LLAMA_TAG` | pinned | `llama.cpp` release tag. |
| `DALI_LLAMA_PARALLEL` | from CPU count | Managed server slots. |
| `DALI_LLAMA_THREADS` | from CPU count | Managed server threads. |
| `DALI_EMBED_CONCURRENCY` | from CPU count | Parallel embedding requests. |
| `DALI_EMBED_BATCH` | `64` | Texts per embedding request. |
| `DALI_LLAMA_KEEP` | `1` | `0` stops the server after each command. |
| `DALI_CACHE_DIR` | `~/.cache/dali` | Download and server cache. |
| `DALI_EMBED_URL` | `http://localhost:11434` | Ollama endpoint. |
| `DALI_EMBED_MODEL` | `bge-m3` | Ollama model. |
| `DALI_EMBED_YES` | | Set to skip the download prompt. |
| `DALI_FIG_DIR` | current directory | Where `.fig` files are discovered. |

### Incremental indexing rules

`dali index` builds only the structural index (SQLite + FTS5), which is fast and
needs no model. Add `--embeddings` to also build the vector index:

```bash
dali index                     # structural only
dali index --embeddings        # structural + vector
dali index --embeddings --yes  # no download prompt
dali index --force             # rebuild even if up to date
```

`dali semantic-search` and the MCP `semantic_search_figma` tool still ensure the
embeddings they need automatically, so `index --embeddings` is only needed to
pre-build them.

- Only nodes whose semantic text changed are re-embedded.
- Nodes with an auto-generated name and no text (for example `Vector` or
  `Frame 12`) are not embedded: they add noise without signal. They remain
  available for structural search.
- Changing the model or bumping `semantic.GeneratorVersion` drops and rebuilds
  the vector index; everything else is incremental.

### Server lifecycle and speed

The managed server is left running after a command, so the next
`dali semantic-search` reuses it and skips the model load:

```bash
dali semantic-search "payment screen"   # slow: downloads and starts the server
dali semantic-search "buttons"          # fast: reuses the warm server
dali serve                              # keep it alive between sessions
dali stop                               # stop it when you are done
```

Set `DALI_LLAMA_KEEP=0` to stop the server after every command. Embedding runs
through a worker pool sized from the machine, feeding a single SQLite writer in
one transaction per batch.

## MCP

### Server

`dali design.fig mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io)
server over stdio. Tool names are prefixed with the server name in clients, so
`search_figma` appears as `dali_search_figma`.

| Tool | Arguments | Description |
| --- | --- | --- |
| `search_figma` | `file?`, `query`, `limit?` | Full-text search. |
| `semantic_search_figma` | `file?`, `query`, `limit?`, `type?`, `page?` | Semantic (vector) search; auto-indexes missing embeddings. |
| `get_node` | `file?`, `id` | Get one node by id. |
| `get_context` | `file?`, `id` | Path, parent, children, component, styles, assets. |
| `get_children` | `file?`, `id` | Direct children. |
| `get_parent` | `file?`, `id` | Parent node. |
| `find_components` | `file?`, `name?` | List components, optionally filtered by name substring. |
| `find_assets` | `file?`, `name?` | List image assets, optionally filtered by name substring. |
| `find_text` | `file?`, `query`, `limit?` | Search text content of TEXT nodes. |
| `list_figs` | | List `.fig`/`.deck` files in the current directory. |

Every tool accepts an optional `file` argument.

### Multiple files

One server serves many designs; each tool takes `file` to pick one. If you omit
it, the fig passed to `dali mcp` is used, and if there is none, the single
`.fig` in the working directory. You can start the server with no file:

```bash
dali mcp
```

### Configuration

Generate the exact file for your editor with absolute paths:

```bash
dali mcp-config                                  # all project-level editors
dali design.fig mcp-config --all                 # ...and Windsurf (global)
dali design.fig mcp-config --claude              # Claude Code (.mcp.json)
dali design.fig mcp-config --cursor              # Cursor (.cursor/mcp.json)
dali design.fig mcp-config --opencode            # OpenCode (opencode.json)
dali design.fig mcp-config --vscode              # VS Code (.vscode/mcp.json)
dali design.fig mcp-config --cline               # Cline (.cline/mcp.json)
dali design.fig mcp-config --roo                 # Roo Code (.roo/mcp.json)
dali mcp-config --windsurf                       # Windsurf (global)
dali mcp-config --src ~/designs/design.fig       # point at a design anywhere
dali mcp-config --print --vscode                 # print instead of writing
```

Each editor reads its own file, all following their documented convention:

| Editor | Config file | Root key |
| --- | --- | --- |
| OpenCode | `opencode.json` | `mcp` |
| Cursor | `.cursor/mcp.json` | `mcpServers` |
| Claude Code | `.mcp.json` | `mcpServers` |
| VS Code | `.vscode/mcp.json` | `servers` |
| Cline | `.cline/mcp.json` | `mcpServers` |
| Roo Code | `.roo/mcp.json` | `mcpServers` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | `mcpServers` |

Configs are written from the directory where you run the command (override with
`--dir`), so the editor picks them up when it opens that folder. Existing files
are **merged**, not replaced: the `dali` server is added or updated under the
root key and any other servers or settings you already had are preserved.
Windsurf is the exception — it only supports a global config file, so it is not
written by default.

The design file path inside each config is absolute, so it resolves no matter
where the editor runs from.

**OpenCode** — `opencode.json`:

```json
{
  "mcp": {
    "dali": {
      "type": "local",
      "command": ["/absolute/path/dali", "/absolute/path/design.fig", "mcp"],
      "environment": { "DALI_EMBED_YES": "1" },
      "enabled": true,
      "timeout": 120000
    }
  }
}
```

**Cursor** — `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "dali": {
      "command": "/absolute/path/dali",
      "args": ["/absolute/path/design.fig", "mcp"],
      "env": { "DALI_EMBED_YES": "1" }
    }
  }
}
```

**Claude Code** — `.mcp.json`:

```json
{
  "mcpServers": {
    "dali": {
      "type": "stdio",
      "command": "/absolute/path/dali",
      "args": ["/absolute/path/design.fig", "mcp"],
      "env": { "DALI_EMBED_YES": "1" }
    }
  }
}
```

**VS Code** — `.vscode/mcp.json`:

```json
{
  "servers": {
    "dali": {
      "type": "stdio",
      "command": "/absolute/path/dali",
      "args": ["/absolute/path/design.fig", "mcp"],
      "env": { "DALI_EMBED_YES": "1" }
    }
  }
}
```

**Cline** — `.cline/mcp.json` and **Roo Code** — `.roo/mcp.json` share the same
shape (Cline uses `mcpServers`, Roo Code `mcpServers`):

```json
{
  "mcpServers": {
    "dali": {
      "command": "/absolute/path/dali",
      "args": ["/absolute/path/design.fig", "mcp"],
      "env": { "DALI_EMBED_YES": "1" },
      "disabled": false,
      "alwaysAllow": []
    }
  }
}
```

Two things avoid the common failures:

- `DALI_EMBED_YES=1` lets semantic search download the model. Over stdio there
  is no terminal to confirm, so without it the first `semantic_search_figma`
  fails. Alternatively run `dali design.fig index --yes` once beforehand.
- OpenCode's `timeout` defaults to 5000 ms. The first start (parsing the fig
  into the index) can exceed it, so raise it.

#### Indexing feedback

When a tool opens a fig whose index is missing or stale, the server rebuilds it
on the fly. Clients that send an `_meta.progressToken` receive incremental
`notifications/progress` (`indexing design.fig` → `indexed design.fig`, and
`embedding (done/total)` for semantic search); regardless of the client, every
event is also logged to **stderr** as `dali: …` lines, which editors surface in
the MCP server logs.

### Zero-install launcher

To avoid installing the binary, point the client at `run.sh`. It caches the
release binary on first use and keeps stdout clean for the MCP stream:

```json
"command": ["sh", "-c", "curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/run.sh | sh -s -- /absolute/path/design.fig mcp"]
```

## Generated files

For `design.fig`:

```text
design.fig
design.assets/              # extracted assets (user-facing)
    images/
.dali/                      # technical files
    design.fig.db           # SQLite index (rebuildable)
    design.fig.db-wal
    design.fig.db-shm
```

`.dali/` holds only technical files and is disposable: delete it and Dalí
rebuilds the index from the `.fig`. User-facing output — extracted assets and
future exports — stays next to the `.fig`.

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
| `parser/` | ZIP and chunk decompression (raw-deflate or zstd) into a raw message. |
| `model/` | Node tree built from the flat `nodeChanges` array. |
| `index/` | SQLite schema, writes, FTS5, and the `vec_nodes` vector table. |
| `query/` | All reads; owns SQL. Tree, search, semantic search, context. |
| `embed/` | `Embedder` interface, OpenAI-compatible client, managed llama.cpp. |
| `semantic/` | Semantic text per node and the incremental embedding index. |
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

No build tags are needed: FTS5 and `sqlite-vec` are compiled into
`modernc.org/sqlite` at the pinned version. (Older versions of that dependency
required `-tags sqlite_fts5`.)

The version embedded in `dali version` is set at build time:

```bash
go build -ldflags "-X main.version=v1.2.3" -o dali ./cmd/dali
```

Integration tests and the parser benchmark use `../woments.fig` and skip when it
is absent. Live tests are opt-in: `DALI_NET_TEST=1` (GitHub/HuggingFace
resolution) and `DALI_MANAGED_TEST=1` (downloads the engine and model).

## Contributing

- Keep the layer separation: `parser` → `model` → `index` → `query`, with the
  CLI and MCP as thin consumers. All read SQL lives in `query/`.
- Add tests with the fake embedder for anything that touches embeddings; live
  network tests stay behind `DALI_NET_TEST` / `DALI_MANAGED_TEST`.
- Run `go test ./...` and `go vet ./...` before opening a PR.
- See [AGENTS.md](AGENTS.md) for architecture details and conventions.

## License

MIT. See [LICENSE](LICENSE).