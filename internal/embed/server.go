package embed

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

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
		"LD_LIBRARY_PATH="+dir+string(os.PathListSeparator)+os.Getenv("LD_LIBRARY_PATH"),
		"DYLD_LIBRARY_PATH="+dir+string(os.PathListSeparator)+os.Getenv("DYLD_LIBRARY_PATH"))
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

func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
