package embed

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/salvadorsru/dali/internal/output"
)

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
	_, statErr := os.Stat(modelPath)
	needModel := statErr != nil && os.IsNotExist(statErr)

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
		URL:   srv.URL,
		Tag:   m.tag(),
		Model: m.Model(),
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

func (m *Managed) confirm(summary string) bool {
	if m.Confirm == nil {
		return false
	}
	return m.Confirm(summary)
}

var _ io.Closer = (*Managed)(nil)