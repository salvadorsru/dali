package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	// Prefer a small, fast quant: the download is much smaller than Q8_0 and
	// embedding quality holds up well for design-text retrieval.
	quants := []string{"Q4_K_M", "Q4_K_S", "Q8_0"}
	for _, q := range quants {
		for i := range model.Siblings {
			s := &model.Siblings[i]
			if strings.HasSuffix(strings.ToLower(s.Filename), ".gguf") && strings.Contains(s.Filename, q) {
				return *s, nil
			}
		}
	}
	for i := range model.Siblings {
		s := &model.Siblings[i]
		if !strings.HasSuffix(strings.ToLower(s.Filename), ".gguf") {
			continue
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