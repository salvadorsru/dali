package embed

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// serverInfo is the record of a managed llama-server left running so later
// commands can reuse it instead of paying the model load cost again.
type serverInfo struct {
	URL     string `json:"url"`
	PID     int    `json:"pid"`
	Tag     string `json:"tag"`
	Model   string `json:"model"`
	Started string `json:"started"`
}

// CacheDir is where Dalí keeps downloaded engines, models and the running
// server record.
func CacheDir() string {
	if v := os.Getenv("DALI_CACHE_DIR"); v != "" {
		return v
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "dali")
}

func serverFile(dir string) string { return filepath.Join(dir, "server.json") }

func readServer(dir string) (serverInfo, bool) {
	var info serverInfo
	data, err := os.ReadFile(serverFile(dir))
	if err != nil {
		return serverInfo{}, false
	}
	if json.Unmarshal(data, &info) != nil || info.URL == "" {
		return serverInfo{}, false
	}
	return info, true
}

func writeServer(dir string, info serverInfo) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(serverFile(dir), data, 0o644)
}

func removeServer(dir string) { _ = os.Remove(serverFile(dir)) }

// keepFromEnv reports whether the managed server should stay running after a
// command. Default true; set DALI_LLAMA_KEEP=0 to stop it every time.
func keepFromEnv() bool {
	return os.Getenv("DALI_LLAMA_KEEP") != "0"
}

func healthy(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// StopServer stops a managed server left running by a previous command.
func StopServer(dir string) error {
	if dir == "" {
		dir = CacheDir()
	}
	info, ok := readServer(dir)
	if !ok {
		return nil
	}
	if p, err := os.FindProcess(info.PID); err == nil {
		_ = p.Kill()
	}
	removeServer(dir)
	return nil
}
