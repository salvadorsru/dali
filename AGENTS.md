# Dalí

**Local search and indexing engine for Figma `.fig` files.**

User-facing docs live in **README.md**. This file is the agent working
contract: what to touch, what not to reshape, and the rules that bite.

Dalí parses `.fig` files on disk, builds a **rebuildable** local SQLite index
(structural, FTS5 full-text, vector) and exposes search through a CLI and an
MCP server. The `.fig` stays the source of truth; `.dali/` is a generated
index you can delete at any time. CLI and MCP share the same `query` layer.

---

## 🏗️ Architecture

```text
.fig → parser.Open → model.Build → index (SQLite/FTS5/vector) → query → CLI/MCP
```

| Package      | Responsibility                                      |
| ------------ | --------------------------------------------------- |
| `kiwi/`      | Kiwi schema decoding and dynamic message decoding   |
| `parser/`    | ZIP/chunk decompression and raw message parsing     |
| `model/`     | Build the document tree from `nodeChanges`          |
| `index/`     | SQLite schema, writes, FTS5, and vector storage     |
| `query/`     | Read-side query API and SQL ownership               |
| `embed/`     | Embedding interface and embedding providers         |
| `semantic/`  | Semantic text generation and incremental embeddings |
| `quality/`   | Design linting and geometry analysis                |
| `codegen/`   | HTML + Tailwind code generation (IR, generator, tokens) |
| `assets/`    | Image extraction                                    |
| `workspace/` | `.dali/` path layout                                |
| `output/`    | Output handling                                     |
| `mcp/`       | MCP server                                          |
| `cmd/dali/`  | CLI entry point                                     |

### Scope

The core is `kiwi/` → `parser/` → `model/` → `index/` → `query/`, plus
`embed/`, `semantic/`, `mcp/` and `cmd/dali/`. That is where correctness and
performance work belongs.

`quality/` and `codegen/` form a **peripheral perimeter**: they are maintained
and must keep working, but are not the focus. Treat their output as frozen —
do not change public behaviour unless the task asks, and never let perimeter
work reshape the core pipelines. They read `.fig` through
`parser.Open` + `model.Build` directly (conformance test in
`query/conformance_test.go`), so a core model change can still surface there;
keep that path covered.

---

## 🔎 Search

Three complementary search modes, one entry point.

### Routing

`search_figma` is the only search entry point. `mode` chooses evaluation:
`text` (default) = exact keyword over node names and text, with near-miss name
suggestions when nothing matches; `semantic` = conceptual meaning search (needs
the vector index, works without exact text); `hybrid` = fuse full-text,
semantic and structural search into one ranked list.

`query_figma` / `xpath_figma` are structural and never use embeddings.

### Tool contracts

| Tool | Embeddings | Indexes |
| --- | --- | --- |
| structural search, `search_figma` `mode=text` | never | never |
| `search_figma` — `mode=hybrid` | only when the vector index already exists and embeddings are ON | never (falls back to full-text) |
| `search_figma` — `mode=semantic` | only when embeddings are ON and the vector index exists | never |
| `index_embeddings` | yes | yes — the only tool that does |

Searches never build embeddings. `index_embeddings` is the only tool that
does, and only it writes the vector index. Exact names → `search_figma`
(`mode=text`) / `query_figma`; near-miss name suggestions live in `query`
(`SuggestNames`). Only use `semantic`/`hybrid` for conceptual queries; both
fail with a hint when disabled or unindexed — call `index_embeddings` first.

---

## 🧠 Semantic Search

`embed.Embedder` abstracts embedding providers. The default manages an official
`llama.cpp` `llama-server` running **`ggml-org/embeddinggemma-300M-GGUF`**,
downloaded to `~/.cache/dali`.

Key rules:

- Downloads are confirmed before starting (skip with `DALI_EMBED_YES=1`).
- The managed server is kept running so subsequent commands reuse it.
- Embeddings use a worker pool and a single SQLite writer.
- The vector index is rebuilt when the model or
  `semantic.GeneratorVersion` changes.

### Configuration

| Variable                 | Purpose                                    |
| ------------------------ | ------------------------------------------ |
| `DALI_EMBEDDINGS=1`      | Opt-in: enable embeddings in the MCP server |
| `DALI_EMBED_PROVIDER`    | `auto`, `ollama`, `llama`, `remote`, `off` |
| `DALI_CACHE_DIR`         | Dalí cache directory                       |
| `DALI_LLAMA_URL`         | Reuse an existing `llama-server`           |
| `DALI_EMBED_URL`         | Ollama embedding endpoint                  |
| `DALI_EMBED_YES=1`       | Skip download confirmation                 |
| `DALI_LLAMA_KEEP=0`      | Stop the managed server after each command |
| `DALI_LLAMA_PARALLEL`    | Override llama-server parallelism          |
| `DALI_LLAMA_THREADS`     | Override llama-server threads              |
| `DALI_EMBED_CONCURRENCY` | Embedding worker concurrency               |
| `DALI_EMBED_BATCH`       | Embedding batch size                       |
| `DALI_NET_TEST=1`        | Enable live network tests                  |
| `DALI_MANAGED_TEST=1`    | Enable managed-server tests                |

---

## 📁 Files

Dalí keeps the original `.fig` untouched:

```text
design.fig
.dali/design.db     rebuildable index (deletable)
design.assets/images/
```

The SQLite DB is a rebuildable index, not the source of truth. Deleting
`.dali/` removes generated data, never the design.

---

## 🖼️ Assets

Images are extracted next to the original file:

```text
design.fig
design.assets/
└── images/
```

---

## 🌱 Lint & Analyze

`quality/` parses via `parser.Open` + `model.Build` directly (CLI and MCP);
it never reads the SQLite index, so results are never stale.

- `quality.Lint` runs presets: `recommended` (default), `strict`, `accessibility`.
- `quality.Analyze` covers `spacing`, `clusters`, `overlaps`. Colour/typography
  aggregation lives in `query` (`get_tokens`); do not duplicate.

---

## ⚙️ Component Codegen

`codegen/entry.go` drives the single `codegen/html.HTMLGenerator` directly; no
registry or per-framework generators exist.

- `Rotation` is already in degrees; do not re-convert.
- Text into HTML must be escaped, never interpolated raw (e.g. `-15%`).

```bash
dali design.fig generate
dali design.fig generate --component 1:123 --out ./components
dali design.fig generate --tokens css
```

---

## 🛠️ Development

Build:

```bash
go build -o dali ./cmd/dali
```

Test / vet:

```bash
go test ./...
go vet ./...
```

Integration tests and the parser benchmark use `../dali.fig`; they are skipped
automatically when unavailable.

---

## 🗄️ SQLite & FTS5

`modernc.org/sqlite` carries FTS5 compiled in — no build tag required. Do not
downgrade without checking FTS5 supportable. Vector search uses `sqlite-vec`
(CGo-free, registered through a blank import); the vector index rebuilds when
the model or `semantic.GeneratorVersion` changes.
