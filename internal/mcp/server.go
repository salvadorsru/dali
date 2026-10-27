package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/internal/embed"
	"github.com/salvadorsru/dali/internal/query"
)

// Version is reported to MCP clients. The CLI overlays its build-time version
// before serving; providers and tests may leave it at the dev default.
var Version = "dev"

// notifier mirrors status lines to stderr, which editors surface in server
// logs. It never sends notifications/progress: clients like Cursor inject
// _meta.progressToken (often the JSON-RPC id) without registering it, and
// echoing it back kills the stdio session ("progress notification for an
// unknown token").
type notifier struct {
	out io.Writer
}

func newNotifier(context.Context, *mcp.CallToolRequest) *notifier {
	return &notifier{out: os.Stderr}
}

func (n *notifier) start(message string) {
	fmt.Fprintf(n.out, "dali: %s…\n", message)
}

func (n *notifier) done(message string) {
	fmt.Fprintf(n.out, "dali: %s\n", message)
}

// capacity bounds the open services. Closing one is cheap: reopening a design
// costs ~3 ms measured, so a small cache is enough and keeps memory flat.
const capacity = 4

// Deliberately out of scope: vector indexes stay opt-in and belong to one
// active project, the catalog has a single root, no query fuses projects, and a
// read-only designs folder gets no .dali fallback (only the write error explains it).
type handlers struct {
	fig  string // the design fixed by figma mode, empty in context mode
	root string // designs live below this directory
	emb  embed.Embedder

	mu        sync.Mutex
	active    string                    // selected project in context mode
	catalog   []entry                   // designs under root, filled by designs()
	cataloged bool                      // catalog matches the current tree
	opened    map[string]*query.Service // open services, least used first
	used      []string                  // access order of opened
	locks     map[string]*sync.Mutex    // one open lock per path
}

// Serve runs the MCP server over stdio. A non-empty fig serves that single
// design; otherwise every design under root is served and tool calls name one
// with their "file" argument.
func Serve(ctx context.Context, fig, root string) error {
	s, h := newServer(fig, root, mcpEmbedder())
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

// newServer builds the server for one mode. fig and root are mutually
// exclusive; root is normalised here so callers never depend on the cwd.
func newServer(fig, root string, emb embed.Embedder) (*mcp.Server, *handlers) {
	if fig != "" {
		if abs, err := filepath.Abs(fig); err == nil {
			fig = abs
		}
		root = filepath.Dir(fig)
	} else if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	h := &handlers{
		fig:    fig,
		root:   root,
		emb:    emb,
		opened: make(map[string]*query.Service),
		locks:  make(map[string]*sync.Mutex),
	}
	opts := &mcp.ServerOptions{Instructions: instructions(fig)}
	s := mcp.NewServer(&mcp.Implementation{Name: "dali", Version: Version}, opts)
	registerTools(s, h)
	return s, h
}

// instructions tells the client how to reach a design before the first tool
// call: the mode decides whether one is already mounted or must be chosen.
func instructions(fig string) string {
	if fig != "" {
		return "This server serves one design, mounted when dali started. Never pass the file argument to a tool: every call queries that design."
	}
	return "This server serves the designs under a directory. Call list_figs, then use_figma once with the project name you need; later tool calls default to that project. Switch whenever the user names another project or the work moves; a partial name is enough when it fits one design. If the request could match more than one design, ask the user instead of guessing."
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
	if h.fig == "" {
		mcp.AddTool(s, &mcp.Tool{Name: "list_figs", Description: "List the designs under the designs directory as project identities. Pass one to use_figma to select it. The absolute path is only returned for the active project. Set refresh to rescan the directory."}, h.listFigs)
		mcp.AddTool(s, &mcp.Tool{Name: "use_figma", Description: "Select the project every later tool call defaults to. Call it once after list_figs, and again to switch projects; a partial name resolves when it fits one design. Every response reports the project that answered it."}, h.useFigma)
	}
	mcp.AddTool(s, &mcp.Tool{Name: "lint_figma", Description: "Run design-quality and accessibility checks over the document (contrast, font size, touch targets, generic names, empty frames, duplicate names, unused components). Presets: recommended, strict, accessibility."}, h.lintFigma)
	mcp.AddTool(s, &mcp.Tool{Name: "analyze_figma", Description: "Geometric analysis of the document: spacing (repeated layout gaps), clusters (repeated node sizes), or overlaps (intersecting siblings)."}, h.analyzeFigma)
}

// designs returns the designs under root, scanning the tree only once per
// session until the catalog is invalidated.
func (h *handlers) designs() []entry {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.cataloged {
		h.catalog = discover(h.root)
		h.cataloged = true
	}
	return h.catalog
}

// rescan rebuilds the catalog, so a design added after the session started
// shows up instead of being reported missing.
func (h *handlers) rescan() []entry {
	found := discover(h.root)
	h.mu.Lock()
	h.catalog, h.cataloged = found, true
	h.mu.Unlock()
	return found
}

// resolve turns a tool's optional "file" argument into an absolute path. The
// mode defines the universe: figma mode serves only the mounted design, context
// mode only designs under root.
func (h *handlers) resolve(file string) (string, error) {
	if h.fig != "" {
		return h.mounted(file)
	}
	return h.lookup(file)
}

// mounted accepts the design this server was started on, named by its path or
// by a path resolving to it, and explains the mode for anything else.
func (h *handlers) mounted(file string) (string, error) {
	if file == "" {
		return h.fig, nil
	}
	for _, candidate := range []string{file, filepath.Join(h.root, file)} {
		if abs, err := filepath.Abs(candidate); err == nil && filepath.Clean(abs) == h.fig {
			return h.fig, nil
		}
	}
	return "", fmt.Errorf("this server serves one design, %s, so \"file\" cannot name %q; omit the argument to query it", h.fig, file)
}

// lookup resolves an identifier in context mode: the active selection, the only
// design when there is just one, or an identifier matching a design under root.
func (h *handlers) lookup(file string) (string, error) {
	id := file
	if id == "" {
		h.mu.Lock()
		id = h.active
		h.mu.Unlock()
	}
	found := h.designs()
	if filepath.IsAbs(filepath.Clean(id)) && !inside(h.root, filepath.Clean(id)) {
		return "", fmt.Errorf("%s is outside %s: this server serves designs under that directory only", id, h.root)
	}
	if id == "" {
		switch len(found) {
		case 0:
			return "", fmt.Errorf("no designs under %s: add a .fig file, or start dali on one", h.root)
		case 1:
			return found[0].path, nil
		}
		return "", fmt.Errorf("no design selected: pass \"file\", or call use_figma once; available: %s", strings.Join(names(found), ", "))
	}
	if path, ok := match(found, h.root, id); ok {
		return path, nil
	}
	if path, ok := match(h.rescan(), h.root, id); ok {
		return path, nil
	}
	return "", h.miss(id, found)
}

// match maps an identifier to a design: its exact identity, a project/file pair
// or a path relative to root, with or without extension, or an absolute path
// inside root. Nothing outside root resolves.
func match(found []entry, root, id string) (string, bool) {
	clean := filepath.Clean(id)
	if filepath.IsAbs(clean) {
		if !inside(root, clean) {
			return "", false
		}
		for _, e := range found {
			if e.path == clean {
				return e.path, true
			}
		}
		return "", false
	}
	slash := filepath.ToSlash(clean)
	trimmed := strings.TrimSuffix(slash, filepath.Ext(slash))
	for _, e := range found {
		if e.name == slash || e.name == trimmed {
			return e.path, true
		}
	}
	// A directory holding exactly one design answers to its own name, so
	// "halibut" and "halibut/halibutest" name the same project. Two designs
	// below it leave the name ambiguous instead of picking one.
	if path, ok := uniquePrefix(found, trimmed); ok {
		return path, true
	}
	// A partial name resolves only when it fits exactly one design, so switching
	// projects stays one call without ever guessing.
	return uniqueSubstring(found, trimmed)
}

// uniquePrefix resolves a directory name to its only design.
func uniquePrefix(found []entry, dir string) (string, bool) {
	only, depth := "", strings.Count(dir, "/")+1
	for _, e := range found {
		if !strings.HasPrefix(e.name, dir+"/") || strings.Count(e.name, "/") != depth {
			continue
		}
		if only != "" {
			return "", false
		}
		only = e.path
	}
	return only, only != ""
}

// uniqueSubstring resolves a partial identity that matches exactly one design.
func uniqueSubstring(found []entry, needle string) (string, bool) {
	if needle == "" {
		return "", false
	}
	needle = strings.ToLower(needle)
	only := ""
	for _, e := range found {
		if !strings.Contains(strings.ToLower(e.name), needle) {
			continue
		}
		if only != "" {
			return "", false
		}
		only = e.path
	}
	return only, only != ""
}

// project names the design that answers a call, so a wrong selection shows up
// in the response instead of surfacing later as confusing results.
func (h *handlers) project(file string) string {
	abs, err := h.resolve(file)
	if err != nil {
		return ""
	}
	return identity(h.root, abs)
}

// elsewhere suggests switching projects when a query matched nothing and the
// catalog holds more than one design. Empty when there is nowhere else to look.
func (h *handlers) elsewhere(file string) string {
	found := h.designs()
	if len(found) < 2 {
		return ""
	}
	here := h.project(file)
	var other []string
	for _, e := range found {
		if e.name != here && len(other) < 5 {
			other = append(other, e.name)
		}
	}
	if len(other) == 0 {
		return ""
	}
	msg := "no matches in " + here + "; other projects: " + strings.Join(other, ", ")
	if len(found)-1 > len(other) {
		msg += " (list_figs shows the rest)"
	}
	return msg + " — switch with use_figma"
}

// inside reports whether path is root or below it, so a relative root never
// leaks the rest of the filesystem into the catalog.
func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// names returns the design identities, sorted by discover.
func names(found []entry) []string {
	out := make([]string, 0, len(found))
	for _, e := range found {
		out = append(out, e.name)
	}
	return out
}

// miss explains a failed lookup: the available designs plus up to five names
// sharing a prefix or substring with the identifier. No fuzzy comparison: it is
// quadratic over the catalog and guesses wrong.
func (h *handlers) miss(id string, found []entry) error {
	if len(found) == 0 {
		return fmt.Errorf("no designs under %s: add a .fig file, or start dali on one", h.root)
	}
	msg := fmt.Sprintf("no design %q under %s; available: %s", id, h.root, strings.Join(names(found), ", "))
	if hint := hints(id, found); len(hint) > 0 {
		msg += "; did you mean " + strings.Join(hint, ", ") + "?"
	}
	return errors.New(msg)
}

// hints returns at most five identities that look like the identifier typed:
// prefix matches first, then substring matches, case-folded.
func hints(id string, found []entry) []string {
	needle := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(filepath.Clean(id)), filepath.Ext(id)))
	if needle == "" {
		return nil
	}
	var prefix, contains []string
	for _, e := range found {
		name := strings.ToLower(e.name)
		switch {
		case strings.HasPrefix(name, needle):
			prefix = append(prefix, e.name)
		case strings.Contains(name, needle):
			contains = append(contains, e.name)
		}
	}
	out := append(prefix, contains...)
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// service returns the query service for a fig file, opening and caching it.
// When the structural index is rebuilt on first use, the notifier logs it to
// stderr.
func (h *handlers) service(file string, note *notifier) (*query.Service, error) {
	abs, err := h.resolve(file)
	if err != nil {
		return nil, err
	}
	if svc, ok := h.cached(abs); ok {
		return svc, nil
	}
	lock := h.lock(abs)
	lock.Lock()
	defer lock.Unlock()
	if svc, ok := h.cached(abs); ok {
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
	h.store(abs, svc)
	return svc, nil
}

// lock returns the open lock for a path, creating it under the short lock. The
// lock is held across query.OpenWith, so indexing one project never blocks
// queries to another.
func (h *handlers) lock(abs string) *sync.Mutex {
	h.mu.Lock()
	defer h.mu.Unlock()
	lock, ok := h.locks[abs]
	if !ok {
		lock = &sync.Mutex{}
		h.locks[abs] = lock
	}
	return lock
}

// cached returns an open service and marks it as most recently used.
func (h *handlers) cached(abs string) (*query.Service, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	svc, ok := h.opened[abs]
	if ok {
		h.touch(abs)
	}
	return svc, ok
}

// store caches a service and evicts the least recently used one, never the
// active project. Callers must hold the lock for abs.
func (h *handlers) store(abs string, svc *query.Service) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.opened[abs] = svc
	h.touch(abs)
	for len(h.opened) > capacity {
		victim := ""
		for _, path := range h.used {
			if path != h.active {
				victim = path
				break
			}
		}
		if victim == "" {
			return
		}
		h.opened[victim].Close()
		delete(h.opened, victim)
		h.used = slices.DeleteFunc(h.used, func(path string) bool { return path == victim })
	}
}

// touch moves a path to the most recently used end of the access order.
func (h *handlers) touch(abs string) {
	h.used = slices.DeleteFunc(h.used, func(path string) bool { return path == abs })
	h.used = append(h.used, abs)
}

func (h *handlers) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, svc := range h.opened {
		svc.Close()
	}
	h.opened = nil
	h.used = nil
	_ = embed.Close(h.emb)
}
