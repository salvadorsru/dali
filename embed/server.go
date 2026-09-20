package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/salvadorsru/dali/output"
)

const (
	DefaultLlamaTag  = "b10659"
	llamaReleaseRepo = "ggml-org/llama.cpp"
	DefaultModelRepo = "ggml-org/embeddinggemma-300M-GGUF"
)

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type hfSibling struct {
	Filename string `json:"rfilename"`
	LFS      *struct {
		OID    string `json:"oid"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	} `json:"lfs"`
}

func (s hfSibling) sha() string {
	if s.LFS == nil {
		return ""
	}
	if s.LFS.OID != "" {
		return s.LFS.OID
	}
	return s.LFS.SHA256
}

type hfModel struct {
	Siblings []hfSibling `json:"siblings"`
}

func getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "dali")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func archName(goarch string) string {
	if goarch == "amd64" {
		return "x64"
	}
	return goarch
}

func assetSuffix(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		return "bin-macos-" + archName(goarch), nil
	case "linux":
		return "bin-ubuntu-" + archName(goarch), nil
	case "windows":
		return "bin-win-cpu-" + archName(goarch), nil
	}
	return "", fmt.Errorf("no prebuilt llama.cpp for %s/%s", goos, goarch)
}

func resolveAsset(ctx context.Context, tag, goos, goarch string) (releaseAsset, error) {
	suffix, err := assetSuffix(goos, goarch)
	if err != nil {
		return releaseAsset{}, err
	}
	var rel struct {
		Assets []releaseAsset `json:"assets"`
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", llamaReleaseRepo, tag)
	if err := getJSON(ctx, url, &rel); err != nil {
		return releaseAsset{}, err
	}
	for _, a := range rel.Assets {
		if strings.Contains(a.Name, suffix) {
			return a, nil
		}
	}
	return releaseAsset{}, fmt.Errorf("no asset matching %q in llama.cpp %s", suffix, tag)
}

func resolveModel(ctx context.Context, repo, wantFile string) (hfSibling, error) {
	var model hfModel
	url := fmt.Sprintf("https://huggingface.co/api/models/%s?blobs=true", repo)
	if err := getJSON(ctx, url, &model); err != nil {
		return hfSibling{}, err
	}
	if wantFile != "" {
		for _, s := range model.Siblings {
			if s.Filename == wantFile {
				return s, nil
			}
		}
	}
	var fallback *hfSibling
	for i := range model.Siblings {
		s := &model.Siblings[i]
		if !strings.HasSuffix(strings.ToLower(s.Filename), ".gguf") {
			continue
		}
		if strings.Contains(s.Filename, "Q8_0") {
			return *s, nil
		}
		if fallback == nil {
			fallback = s
		}
	}
	if fallback == nil {
		return hfSibling{}, fmt.Errorf("no .gguf file in %s", repo)
	}
	return *fallback, nil
}

func shaFromDigest(digest string) string {
	return strings.TrimPrefix(digest, "sha256:")
}

func findBinary(dir, name string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := d.Name()
		if base == name || base == name+".exe" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found under %s", name, dir)
	}
	return found, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

type Server struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	URL    string
	logs   *bytes.Buffer
	done   chan struct{}
}

func (s *Server) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		<-s.done
	}
	return nil
}

// startServer launches llama-server, trying the tuned flags first and falling
// back to a minimal, maximally compatible configuration if that fails.
func startServer(bin, model string) (*Server, error) {
	parallel := Concurrency()
	ctxSize := parallel * 512
	if ctxSize < 1024 {
		ctxSize = 1024
	}
	threads := Threads()
	configs := [][]string{
		{"-c", strconv.Itoa(ctxSize), "--parallel", strconv.Itoa(parallel),
			"-t", strconv.Itoa(threads), "-tb", strconv.Itoa(threads)},
		{"-c", strconv.Itoa(ctxSize), "--parallel", strconv.Itoa(parallel)},
		{"-c", "512"},
	}

	var lastErr error
	for _, extra := range configs {
		srv, err := launch(bin, model, extra)
		if err == nil {
			return srv, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func launch(bin, model string, extra []string) (*Server, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(bin)
	ctx, cancel := context.WithCancel(context.Background())
	args := append([]string{
		"-m", model,
		"--embeddings", "--pooling", "mean", "--embd-normalize", "2",
		"--host", "127.0.0.1", "--port", strconv.Itoa(port), "--no-webui",
	}, extra...)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH="+dir+pathListSeparator()+os.Getenv("LD_LIBRARY_PATH"),
		"DYLD_LIBRARY_PATH="+dir+pathListSeparator()+os.Getenv("DYLD_LIBRARY_PATH"))
	logs := &bytes.Buffer{}
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	srv := &Server{
		cmd:    cmd,
		cancel: cancel,
		URL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		logs:   logs,
		done:   make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(srv.done)
	}()
	if err := srv.waitReady(90 * time.Second); err != nil {
		srv.Close()
		return nil, fmt.Errorf("%w\n%s", err, tail(logs.String(), 2000))
	}
	return srv, nil
}

func (s *Server) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-s.done:
			return fmt.Errorf("llama-server exited")
		default:
		}
		resp, err := http.Get(s.URL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("llama-server did not become ready")
}

func pathListSeparator() string {
	if runtime.GOOS == "windows" {
		return ";"
	}
	return ":"
}

func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

// Managed runs an official llama.cpp server on a downloaded GGUF model.
type Managed struct {
	CacheDir  string
	Tag       string
	ModelRepo string
	ModelFile string
	Confirm   func(summary string) bool

	// Keep leaves the server running after Close so the next command reuses it.
	Keep bool

	mu     sync.Mutex
	server *Server
	client *OpenAI
	owned  bool
}

func (m *Managed) Model() string {
	if m.ModelFile != "" {
		return m.modelRepo() + ":" + m.ModelFile
	}
	return m.modelRepo()
}

func (m *Managed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client == nil {
		if err := m.start(ctx); err != nil {
			return nil, err
		}
	}
	return m.client.Embed(ctx, texts)
}

// Serve starts the managed server and blocks until ctx is cancelled, then stops
// it. It is the explicit, foreground way to keep a server warm.
func (m *Managed) Serve(ctx context.Context) error {
	m.mu.Lock()
	m.Keep = false
	if err := m.start(ctx); err != nil {
		m.mu.Unlock()
		return err
	}
	url := ""
	if m.client != nil {
		url = m.client.BaseURL
	}
	m.mu.Unlock()

	if url != "" {
		fmt.Fprintf(os.Stderr, "dali: llama-server listening at %s\n", url)
	}
	<-ctx.Done()
	return m.Close()
}

func (m *Managed) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	srv := m.server
	if srv == nil {
		return nil
	}
	m.server = nil
	m.client = nil
	if !m.owned {
		return nil
	}
	if m.Keep {
		// Leave it running for the next command; server.json stays valid.
		return nil
	}
	removeServer(m.cacheDir())
	return srv.Close()
}

func (m *Managed) cacheDir() string {
	if m.CacheDir != "" {
		return m.CacheDir
	}
	return CacheDir()
}

func (m *Managed) tag() string {
	if m.Tag != "" {
		return m.Tag
	}
	return DefaultLlamaTag
}

func (m *Managed) modelRepo() string {
	if m.ModelRepo != "" {
		return m.ModelRepo
	}
	return DefaultModelRepo
}

func (m *Managed) start(ctx context.Context) error {
	root := m.cacheDir()

	// Reuse a server left running by a previous command.
	if info, ok := readServer(root); ok {
		if info.Model == m.Model() && info.Tag == m.tag() && healthy(info.URL) {
			m.client = NewOpenAI(info.URL, m.Model())
			return nil
		}
		_ = StopServer(root)
	}

	binDir := filepath.Join(root, "llama", m.tag())

	binPath, findErr := findBinary(binDir, "llama-server")
	needBin := findErr != nil
	var asset releaseAsset
	if needBin {
		var err error
		if asset, err = resolveAsset(ctx, m.tag(), runtime.GOOS, runtime.GOARCH); err != nil {
			return err
		}
	}

	sibling, err := resolveModel(ctx, m.modelRepo(), m.ModelFile)
	if err != nil {
		return err
	}
	modelPath := filepath.Join(root, "models", sibling.Filename)
	needModel := !exists(modelPath)

	if needBin || needModel {
		var lines []string
		if needBin {
			lines = append(lines, fmt.Sprintf("  engine: %s (%.1f MB)\n          %s\n          sha256:%s",
				asset.Name, float64(asset.Size)/1e6, asset.URL, shaFromDigest(asset.Digest)))
		}
		if needModel {
			sha, size := sibling.sha(), int64(0)
			if sibling.LFS != nil {
				size = sibling.LFS.Size
			}
			lines = append(lines, fmt.Sprintf("  model:  %s (%.1f MB)\n          %s\n          sha256:%s",
				sibling.Filename, float64(size)/1e6, hfURL(m.modelRepo(), sibling.Filename), sha))
		}
		if !m.confirm("Dalí needs to download:\n" + strings.Join(lines, "\n")) {
			return fmt.Errorf("download declined")
		}
	}

	if needBin {
		archive := filepath.Join(root, asset.Name)
		if err := download(ctx, asset.URL, archive, shaFromDigest(asset.Digest)); err != nil {
			return err
		}
		if err := extractArchive(archive, binDir); err != nil {
			return err
		}
		binPath, findErr = findBinary(binDir, "llama-server")
		if findErr != nil {
			return findErr
		}
	}

	if needModel {
		if err := download(ctx, hfURL(m.modelRepo(), sibling.Filename), modelPath, sibling.sha()); err != nil {
			return err
		}
	}

	status := output.Start("starting llama-server")
	srv, err := startServer(binPath, modelPath)
	status.Stop()
	if err != nil {
		return err
	}
	m.server = srv
	m.owned = true
	m.client = NewOpenAI(srv.URL, m.Model())
	info := serverInfo{
		URL:     srv.URL,
		Tag:     m.tag(),
		Model:   m.Model(),
		Started: time.Now().Format(time.RFC3339),
	}
	if srv.cmd != nil && srv.cmd.Process != nil {
		info.PID = srv.cmd.Process.Pid
	}
	_ = writeServer(root, info)
	return nil
}

func hfURL(repo, file string) string {
	return fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, file)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (m *Managed) confirm(summary string) bool {
	if m.Confirm == nil {
		return false
	}
	return m.Confirm(summary)
}

var _ io.Closer = (*Managed)(nil)
