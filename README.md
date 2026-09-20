# Dalí

Dalí reads a Figma `.fig` file and turns it into a queryable knowledge base: it
parses the document, indexes it in SQLite, and exposes the result to a CLI and
to AI agents over MCP.

You point it at a `.fig` and ask questions — full-text, structural, or
semantic — instead of opening the file by hand.

```
.fig ──► parser ──► model ──► index (SQLite + FTS5 + vec0)
                                   │
                                   ▼
                             query.Service
                             ┌─────┴─────┐
                           CLI          MCP
```

`.fig` is the source of truth. SQLite is a rebuildable index. `query.Service`
is the core; the CLI and the MCP server are consumers of the same engine.

## Features

- Reads `.fig` and `.deck` archives (ZIP + fig-kiwi binary) with a
  self-describing Kiwi decoder.
- Rebuilds the node tree from the flat `nodeChanges` array.
- Indexes nodes, pages, components, instances, assets, styles and fonts.
- Full-text search (FTS5) over node names and text.
- Semantic search with `sqlite-vec` (`vec0`) and embeddings.
- Structural queries: `type=`, `name~`, `descendants()`, `ancestors()`, `children()`.
- Extracts image assets to disk.
- MCP server for AI agents, with multiple files per server.
- Incremental: unchanged nodes are never re-embedded.
- No CGO. SQLite and `sqlite-vec` are pure Go; the embedding engine is a
  managed `llama.cpp` binary downloaded on first use.

## Install

Prebuilt binaries for Linux, macOS and Windows (amd64 and arm64) are on the
[releases page](https://github.com/salvadorsru/dali/releases).

```bash
curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/install.sh | sh
```

The installer verifies the SHA256 and puts `dali` in `~/.local/bin`.

With Go installed:

```bash
go install github.com/salvadorsru/dali/cmd/dali@latest
```

To run without installing anything, use the launcher. It caches the binary on
first use and reuses it afterwards (like `npx` or `uvx`):

```bash
curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/run.sh | sh -s -- design.fig inspect
```

## Quick start

```bash
dali design.fig inspect
dali design.fig search "checkout"
dali design.fig semantic-search "payment screen"
dali design.fig mcp-config --opencode
```

The file is optional. When omitted, Dalí uses the single `.fig` in the current
directory, and a bare `dali` lists what it found:

```bash
cd ~/designs/my-project
dali                      # Found design.fig. Try: ...
dali inspect
dali semantic-search "payment screen"
```

The first semantic search downloads the official `llama.cpp` engine and the
`ggml-org/embeddinggemma-300M` model (about 350 MB) into `~/.cache/dali`, after
showing the URLs, sizes and SHA256 hashes.

## CLI

```
dali [<file.fig>] <command> [args]
```

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
| `query <expr>` | Structural query. |
| `index [--embeddings]` | Build the structural index; `--embeddings` adds the vector index (`--force` rebuilds). |
| `extract-assets` | Write images to `<base>.assets/images`. |
| `mcp` | Run the MCP server over stdio. |
| `mcp-config` | Print MCP config (`--opencode`, `--vscode`; default `mcpServers`). |
| `serve` | Keep the embedding server running. |
| `stop` | Stop the managed embedding server. |
| `version` | Print version and build info. |

Global flags: `--json` (JSON output), `--yes` (allow downloads without asking),
`--no-color`, `--quiet`/`-q` (hide progress and status).

Output is styled (colors, progress bars, spinners) when stdout/stderr are a
terminal and `NO_COLOR` is unset; it falls back to plain text otherwise. All
visual output goes to stderr, so `--json` and the MCP stdio stream stay clean.

Node ids use the `sessionID:localID` form, for example `123:456`.

### Semantic search flags

```bash
dali design.fig semantic-search "primary checkout button" --limit 10
dali design.fig semantic-search "payment" --type SYMBOL
dali design.fig semantic-search "login" --page "UI Desktop"
dali design.fig semantic-search "checkout" --hybrid --json
```

| Flag | Description |
| --- | --- |
| `--limit N` | Max results (default 20). |
| `--type T` | Filter by node type (`FRAME`, `TEXT`, `SYMBOL`, ...). |
| `--page P` | Filter by page name or id. |
| `--hybrid` | Fuse full-text and vector results with Reciprocal Rank Fusion. |
| `--yes` | Allow the model download without prompting. |

### Structural query expressions

```bash
dali design.fig query 'type=TEXT'
dali design.fig query 'name~"button"'
dali design.fig query 'descendants("Checkout")'
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

### Incremental and indexing rules

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

### Speed

The managed server is left running after a command, so the next
`dali semantic-search` reuses it and skips the model load:

```bash
dali semantic-search "payment screen"   # ~1.3s: starts the server
dali semantic-search "buttons"          # ~0.1s: reuses it
dali stop                               # stops it when you are done
```

Set `DALI_LLAMA_KEEP=0` to stop the server after every command. Embedding runs
through a worker pool sized from the machine, feeding a single SQLite writer in
one transaction per batch.

## MCP

`dali design.fig mcp` runs a Model Context Protocol server over stdio. Tools
are prefixed with the server name, so `search_figma` becomes
`dali_search_figma`.

- `search_figma` — full-text search.
- `semantic_search_figma` — semantic search (`query`, `limit`, `type?`, `page?`).
- `get_node` — one node by id.
- `get_context` — path, parent, children, component, styles, assets.
- `get_children`, `get_parent`.
- `find_components`, `find_assets`, `find_text`.
- `list_figs` — figs available in the current directory.

### Multiple files

One server serves many designs. Every tool takes an optional `file` argument;
if you omit it, the fig passed to `dali mcp` is used, and if there is none, the
single `.fig` in the working directory. You can start the server with no file:

```bash
dali mcp
```

### Configuration

Generate the exact block for your editor with absolute paths:

```bash
dali design.fig mcp-config             # Claude / Cursor (mcpServers)
dali design.fig mcp-config --opencode  # opencode.json
dali design.fig mcp-config --vscode    # .vscode/mcp.json
```

OpenCode (`~/.config/opencode/opencode.json`):

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

Two things avoid the common failures:

- `DALI_EMBED_YES=1` lets semantic search download the model. Over stdio there
  is no terminal to confirm, so without it the first `semantic_search_figma`
  fails. Alternatively run `dali design.fig index --yes` once beforehand.
- `timeout` defaults to 5000 ms in OpenCode. The first start (parsing the fig
  into the index) can exceed it, so raise it.

### Zero-install launcher

To avoid installing the binary, point the client at `run.sh`. It caches the
release binary on first use and keeps stdout clean for the MCP stream:

```json
"command": ["sh", "-c", "curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/run.sh | sh -s -- /absolute/path/design.fig mcp"]
```

## Generated files

For `design.fig`:

```
design.fig
design.assets/         # extracted assets (user-facing)
    images/
.dali/                 # technical files
    design.fig.db      # SQLite index (rebuildable)
    design.fig.db-wal
    design.fig.db-shm
```

`.dali/` holds only technical files and is disposable: delete it and Dalí
rebuilds the index from the `.fig`. User-facing output — extracted assets and
future exports — stays next to the `.fig`.

## Architecture

```
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

No build tags are needed: FTS5 is compiled into `modernc.org/sqlite` at the
pinned version. (Older versions of that dependency required `-tags sqlite_fts5`.)

Integration tests and the parser benchmark use `../woments.fig` and skip when
it is absent. Live tests are opt-in: `DALI_NET_TEST=1` (GitHub/HuggingFace
resolution) and `DALI_MANAGED_TEST=1` (downloads the engine and model).

## License

MIT. See [LICENSE](LICENSE).
