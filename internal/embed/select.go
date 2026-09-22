package embed

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FromEnv selects an embedder from the environment.
//
//	DALI_EMBED_PROVIDER  auto (default) | ollama | llama | remote | off
//	DALI_LLAMA_URL       use an already-running llama-server (OpenAI API)
//	DALI_EMBED_URL       Ollama endpoint (default http://localhost:11434)
//	DALI_EMBED_MODEL     Ollama model (default bge-m3)
//	DALI_LLAMA_MODEL     managed model repo (default ggml-org/embeddinggemma-300M-GGUF)
//	DALI_LLAMA_FILE      managed model file (optional)
//	DALI_LLAMA_TAG       pinned llama.cpp release tag
//	DALI_CACHE_DIR       download cache (default user cache dir /dali)
//	DALI_EMBED_YES       skip the download confirmation prompt
//
// A provider of "off" (or "none") returns nil, disabling embeddings entirely.
func FromEnv() Embedder {
	switch os.Getenv("DALI_EMBED_PROVIDER") {
	case "off", "none":
		return nil
	case "ollama":
		return ollamaFromEnv()
	case "llama":
		return managedFromEnv()
	case "remote":
		return NewOpenAI(os.Getenv("DALI_EMBED_URL"), env("DALI_EMBED_MODEL", defaultModel))
	}
	// auto
	if url := os.Getenv("DALI_LLAMA_URL"); url != "" {
		return NewOpenAI(url, env("DALI_EMBED_MODEL", defaultModel))
	}
	if url := os.Getenv("DALI_EMBED_URL"); url != "" {
		return &Ollama{URL: url, Name: env("DALI_EMBED_MODEL", defaultModel), Batch: envInt("DALI_EMBED_BATCH", defaultBatch), client: defaultHTTPClient()}
	}
	return managedFromEnv()
}

func ollamaFromEnv() *Ollama {
	return &Ollama{
		URL:    env("DALI_EMBED_URL", defaultURL),
		Name:   env("DALI_EMBED_MODEL", defaultModel),
		Batch:  envInt("DALI_EMBED_BATCH", defaultBatch),
		client: defaultHTTPClient(),
	}
}

func managedFromEnv() *Managed {
	return &Managed{
		CacheDir:  os.Getenv("DALI_CACHE_DIR"),
		Tag:       os.Getenv("DALI_LLAMA_TAG"),
		ModelRepo: os.Getenv("DALI_LLAMA_MODEL"),
		ModelFile: os.Getenv("DALI_LLAMA_FILE"),
		Confirm:   confirmDownload,
		Keep:      keepFromEnv(),
	}
}

// Close releases an embedder's resources (managed server) when it has any.
func Close(emb Embedder) error {
	if c, ok := emb.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

func confirmDownload(summary string) bool {
	fmt.Fprintln(os.Stderr, summary)
	if os.Getenv("DALI_EMBED_YES") != "" {
		return true
	}
	if !isTerminal() {
		fmt.Fprintln(os.Stderr, "dali: not interactive; pass --yes or set DALI_EMBED_YES=1 to allow the download")
		fmt.Fprintln(os.Stderr, providerHint)
		return false
	}
	fmt.Fprint(os.Stderr, "Download now? [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	fmt.Fprintln(os.Stderr, providerHint)
	return false
}

const providerHint = "dali: or reuse an existing embedder with DALI_EMBED_PROVIDER=ollama (Ollama, default bge-m3)"

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
