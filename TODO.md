# Essential Dalí Development Plan

## 1. Incremental Indexing

Make Dalí aware of exactly what changed since the last indexing run.

### Implement

* Per-node `content_hash` / fingerprint.
* Index state and version tracking.
* Detect added, removed, and modified nodes.
* Reuse embeddings that are still valid.
* Invalidate embeddings when their content, model, or `GeneratorVersion` changes.

### Goal

```text
.fig
 ↓
Detect changes
 ↓
Update only what is necessary
```

---

## 2. Unified Query Engine

Make `query.Service` the real core of Dalí.

It should combine:

* structural search;
* FTS5;
* semantic search;
* filters.

Create a single search API instead of keeping each search mechanism isolated.

```text
query.Search(...)
        │
   ┌────┼────┐
   ▼    ▼    ▼
  FTS  Vec  Tree
   │    │    │
   └────┼────┘
        ▼
     Results
```

---

## 3. Hybrid Search + Ranking

This is probably **the main feature Dalí is missing**.

A search such as:

```text
"button to complete the payment"
```

should be able to combine:

* text matching;
* semantic similarity;
* structural relevance.

Then:

```text
Candidates
    ↓
Ranking
    ↓
Top results
```

The initial ranking system does not need to be complicated. The important part is having a clean interface that can evolve later.

---

## 4. Contextual Results

Finding a node should also provide its surrounding context.

A result should be able to include:

```text
node
 ├── page
 ├── ancestors
 ├── parent
 └── relevant children
```

For example:

```text
Checkout
└── Payment
    └── Actions
        └── Continue
```

This is particularly important for MCP: an agent should be able to understand **where a node is and what surrounds it** without having to make several additional queries.

---

## 5. CLI + MCP on Top of the Same Engine

Once the previous pieces are in place:

```text
              query.Service
                    │
          ┌─────────┴─────────┐
          ▼                   ▼
         CLI                 MCP
```

Expose:

* hybrid search;
* node inspection;
* structural context;
* file-specific search.

CLI and MCP should share the same query logic rather than duplicating SQL or search behavior.

---

# Exact Order

```text
1. Fingerprints / index state
             ↓
2. Unified query engine
             ↓
3. Hybrid search
             ↓
4. Ranking
             ↓
5. Contextual results
             ↓
6. CLI + MCP
```

## After the Core Is Done

Once these pieces are working, stop and test Dalí against large `.fig` files and real-world queries.

Only then consider:

* node rendering;
* `diff`;
* advanced property filters;
* cross-file search;
* additional optimizations.

### Target

The goal is for this to work reliably:

```text
User / Agent
       │
       ▼
"Where is the checkout screen
with the button to complete payment?"
       │
       ▼
      Dalí
       │
 ┌─────┼─────┐
 ▼     ▼     ▼
FTS  Vector  Structure
 └─────┼─────┘
       ▼
    Ranking
       ▼
   Context
       ▼
Checkout / Payment / Continue
```

**If this works well, you have the core of Dalí.** Everything else can be built on top of this engine.
