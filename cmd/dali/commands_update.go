package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Overridable so tests serve releases from a local server instead of GitHub.
var (
	releaseBase = "https://github.com/salvadorsru/dali/releases/download"
	latestAPI   = "https://api.github.com/repos/salvadorsru/dali/releases/latest"
)

// cmdUpdate replaces the running binary with the latest published release.
func cmdUpdate(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return update(exe)
}

func update(exe string) error {
	tag, err := latestTag()
	if err != nil {
		return err
	}
	if strings.TrimPrefix(tag, "v") == strings.TrimPrefix(version, "v") {
		fmt.Printf("dali %s is up to date\n", version)
		return nil
	}

	name := assetName()
	base := releaseBase + "/" + tag
	data, err := fetch(base + "/" + name)
	if err != nil {
		return err
	}
	if err := verify(base+"/checksums.txt", name, data); err != nil {
		return err
	}
	bin, err := unpack(data, name)
	if err != nil {
		return err
	}

	fmt.Printf("dali %s -> %s\n", version, tag)
	if err := replace(exe, bin); err != nil {
		return err
	}
	fmt.Printf("updated %s\n", exe)
	return nil
}

func latestTag() (string, error) {
	body, err := fetch(latestAPI)
	if err != nil {
		return "", err
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}
	if release.Tag == "" {
		return "", fmt.Errorf("latest release has no tag: %s", latestAPI)
	}
	return release.Tag, nil
}

// assetName matches the goreleaser archive names: tar.gz everywhere, zip on
// windows.
func assetName() string {
	name := fmt.Sprintf("dali_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// verify checks the archive against the release checksums. A missing checksum
// is an error: the binary is about to replace the running one.
func verify(url, name string, data []byte) error {
	body, err := fetch(url)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && filepath.Base(fields[1]) == name {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum for %s in %s", name, url)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	return nil
}

// unpack returns the dali binary held in the release archive.
func unpack(data []byte, name string) ([]byte, error) {
	if strings.HasSuffix(name, ".zip") {
		return unzip(data)
	}
	return untar(data)
}

func untar(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("archive holds no dali binary")
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && binaryName(h.Name) {
			return io.ReadAll(tr)
		}
	}
}

func unzip(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !binaryName(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("archive holds no dali binary")
}

func binaryName(path string) bool {
	name := filepath.Base(path)
	return name == "dali" || name == "dali.exe"
}

// replace writes the new binary next to the current one and swaps it in. The
// temp file lives in the same directory so the rename stays atomic, and a
// running executable can be replaced everywhere except Windows, where the old
// one is moved aside first.
func replace(exe string, bin []byte) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".dali-update-*")
	if err != nil {
		return fmt.Errorf("cannot write next to %s: %w (try running with sudo)", exe, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
		defer os.Remove(old)
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return err
	}
	return nil
}
