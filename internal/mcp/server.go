package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/internal/embed"
	"github.com/salvadorsru/dali/internal/query"
)

const progressToken = "progressToken"

// Version is reported to MCP clients. The CLI overlays its build-time version
// before serving; providers and tests may leave it at the dev default.
var Version = "dev"

// notifier reports indexing progress to the MCP client as progress
// notifications (only when the client supplies a progress token) and always
// mirrors status lines to stderr, which editors surface in server logs.
type notifier struct {
	ctx     context.Context
	session *mcp.ServerSession
	token   any
}

func newNotifier(ctx context.Context, req *mcp.CallToolRequest) *notifier {
	return &notifier{ctx: ctx, session: req.Session, token: req.Params.Meta[progressToken]}
}

func (n *notifier) start(message string) {
	fmt.Fprintf(os.Stderr, "dali: %s…\n", message)
	n.progress(message, 0, 0)
}

func (n *notifier) done(message string) {
	fmt.Fprintf(os.Stderr, "dali: %s\n", message)
	n.progress(message, 1, 1)
}

func (n *notifier) progress(message string, done, total float64) {
	if n.token == nil {
		return
	}
	_ = n.session.NotifyProgress(n.ctx, &mcp.ProgressNotificationParams{
		ProgressToken: n.token,
		Message:       message,
		Progress:      done,
		Total:         total,
	})
}

type handlers struct {
	def string
	emb embed.Embedder

	mu      sync.Mutex
	opened  map[string]*query.Service
	figures []string // known fig files, for discovery
}

// Serve runs the MCP server over stdio. def is the default fig file; tool calls
// may override it with their own "file" argument, so one server can serve many
// designs.
func Serve(ctx context.Context, def string) error {
	s, h := newServer(def, mcpEmbedder())
	defer h.close()
	return s.Run(ctx, &mcp.StdioTransport{})
}

// mcpEmbedder returns the embedder for the MCP server. Embeddings are opt-in:
// without DALI_EMBEDDINGS=1 no embedder is constructed, so searches stay
// offline and llama-server never starts. When enabled, FromEnv still resolves
// lazily — the managed server only launches on the first Embed call.
func mcpEmbedder() embed.Embedder {
	if os.Getenv("DALI_EMBEDDINGS") == "" {
		return nil
	}
	return embed.FromEnv()
}

func newServer(def string, emb embed.Embedder) (*mcp.Server, *handlers) {
	h := &handlers{
		def:     def,
		emb:     emb,
		opened:  make(map[string]*query.Service),
		figures: discover(),
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "dali", Version: Version}, nil)
	registerTools(s, h)
	return s, h
}

func registerTools(s *mcp.Server, h *handlers) {
	mcp.AddTool(s, &mcp.Tool{Name: "search_figma", Description: "Find nodes across the document. mode chooses how the query is evaluated: text (default) = exact keyword over node names and text content, with near-miss name suggestions when nothing matches; semantic = conceptual meaning search (needs the vector index, works without exact text); hybrid = fuse full-text, semantic and structural search into one ranked list. prefer text/hybrid when you know or can guess the name; only use semantic for conceptual queries with no exact text match. Searches never build embeddings; call index_embeddings first when semantic/hybrid fail with a hint."}, h.search)
	mcp.AddTool(s, &mcp.Tool{Name: "index_embeddings", Description: "Build the vector index: embed every node with semantic content (name or body text) that is not already indexed. This is the only tool that builds embeddings; searches never do. Requires DALI_EMBEDDINGS=1 on the server. Call once after starting dali if you plan semantic or hybrid search."}, h.indexEmbeddings)
	mcp.AddTool(s, &mcp.Tool{Name: "query_figma", Description: "Query the document tree with structural expressions, e.g. type=FRAME, name~\"card\", text~price, visible=true, width>=200, component=Button, children(\"Header\"); combine terms with AND. Leading / queries use the XPath subset, e.g. //FRAME[@name~'card'], /DOCUMENT/CANVAS, //*[contains(@name,'button')]. Suggests near-miss names when nothing matches."}, h.queryFigma)
	mcp.AddTool(s, &mcp.Tool{Name: "get_context", Description: "Get structural context for a node: path, parent, children, component, styles, assets."}, h.getContext)
	mcp.AddTool(s, &mcp.Tool{Name: "get_descendants", Description: "Fetch a whole subtree in one call: every descendant in document order with geometry, styles and images. Optional depth limit. Use this to render a component without repeated get_context round trips."}, h.getDescendants)
	mcp.AddTool(s, &mcp.Tool{Name: "section_figma", Description: "Resolve a named block and inspect it in one call: finds the best layer by name (optionally within a page or under an ancestor), then returns it with its whole subtree (geometry, styles, images, texts), path, parent, near-match alternatives when the name is ambiguous, and line heights in px. Prefer this when asked for a section or component by name."}, h.sectionFigma)
	mcp.AddTool(s, &mcp.Tool{Name: "find_components", Description: "List components, optionally filtered by name substring."}, h.findComponents)
	mcp.AddTool(s, &mcp.Tool{Name: "find_instances", Description: "List component instances, optionally filtered by name substring."}, h.findInstances)
	mcp.AddTool(s, &mcp.Tool{Name: "find_assets", Description: "List image assets, optionally filtered by name substring."}, h.findAssets)
	mcp.AddTool(s, &mcp.Tool{Name: "get_image", Description: "Return a single image asset as base64 data, looked up by hash or name."}, h.getImage)
	mcp.AddTool(s, &mcp.Tool{Name: "extract_assets", Description: "Write images to <base>.assets/images and return the file paths (Optionally filtered by name or hash)."}, h.extractAssets)
	mcp.AddTool(s, &mcp.Tool{Name: "get_styles", Description: "List style definitions (name, kind, resolved value, usage count), optionally filtered by name or kind, or without the raw payload."}, h.getStyles)
	mcp.AddTool(s, &mcp.Tool{Name: "get_tokens", Description: "Aggregate design tokens: solid colour palette, colour variables, fonts, text classes and flex layouts, ranked by usage. Optionally filtered by name (variables by name or set, fonts by family)."}, h.getTokens)
	mcp.AddTool(s, &mcp.Tool{Name: "get_usages", Description: "List every page placement of a component."}, h.getUsages)
	mcp.AddTool(s, &mcp.Tool{Name: "get_dependents", Description: "Impact analysis: the top-level containers (screens) that place a component, with aggregated placement counts, ordered by usage."}, h.getDependents)
	mcp.AddTool(s, &mcp.Tool{Name: "get_component", Description: "Get the codegen contract of a component: variants, direct children, accepted props and placement count."}, h.getComponent)
	mcp.AddTool(s, &mcp.Tool{Name: "generate_figma", Description: "Generate HTML code (inline CSS by default, or Tailwind via the style field) for components and optional design tokens, returned inline for you to write into the project. Omit component and name to generate every component, set component to a single id, or set name to resolve a node by name (ambiguous names return alternatives for disambiguation)."}, h.generateFigma)
	mcp.AddTool(s, &mcp.Tool{Name: "get_stats", Description: "Summarize the document: node counts by type, page and node totals."}, h.getStats)
	mcp.AddTool(s, &mcp.Tool{Name: "list_pages", Description: "List the pages (canvases) of the document in order."}, h.listPages)
	mcp.AddTool(s, &mcp.Tool{Name: "list_figs", Description: "List the .fig/.deck files available in the current directory."}, h.listFigs)
	mcp.AddTool(s, &mcp.Tool{Name: "lint_figma", Description: "Run design-quality and accessibility checks over the document (contrast, font size, touch targets, generic names, empty frames, duplicate names, unused components). Presets: recommended, strict, accessibility."}, h.lintFigma)
	mcp.AddTool(s, &mcp.Tool{Name: "analyze_figma", Description: "Geometric analysis of the document: spacing (repeated layout gaps), clusters (repeated node sizes), or overlaps (intersecting siblings)."}, h.analyzeFigma)
}

// resolve turns a tool's optional "file" argument into an absolute path,
// honouring the default fig and single-file discovery fallbacks.
func (h *handlers) resolve(file string) (string, error) {
	if file == "" {
		file = h.def
	}
	if file == "" {
		if len(h.figures) == 1 {
			file = h.figures[0]
		} else {
			return "", errors.New("no fig file: pass \"file\" in the tool call, or start dali with a fig")
		}
	}
	return filepath.Abs(file)
}

// service returns the query service for a fig file, opening and caching it.
// When the structural index is rebuilt on first use, the notifier reports it.
func (h *handlers) service(file string, note *notifier) (*query.Service, error) {
	abs, err := h.resolve(file)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if svc, ok := h.opened[abs]; ok {
		return svc, nil
	}
	base := filepath.Base(abs)
	svc, err := query.OpenWith(abs, false, func() {
		note.start("indexing " + base)
	})
	if err != nil {
		return nil, err
	}
	if svc.Rebuilt() {
		note.done("indexed " + base)
	}
	h.opened[abs] = svc
	return svc, nil
}

func (h *handlers) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, svc := range h.opened {
		svc.Close()
	}
	h.opened = nil
	_ = embed.Close(h.emb)
}
