package embed

import (
	"runtime"
)

var defaultBatch = 64

const (
	defaultModel = "bge-m3"
	defaultURL   = "http://localhost:11434"
)

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Parallel is the number of concurrent embedding requests and llama-server slots.
func Parallel() int {
	if n := envInt("DALI_LLAMA_PARALLEL", 0); n > 0 {
		return n
	}
	return clamp(runtime.NumCPU()/3, 1, 8)
}

// Threads is the number of CPU threads llama-server uses per request.
func Threads() int {
	if n := envInt("DALI_LLAMA_THREADS", 0); n > 0 {
		return n
	}
	return clamp(runtime.NumCPU()/2, 1, 16)
}

// Concurrency is how many embedding requests run at once.
func Concurrency() int {
	if n := envInt("DALI_EMBED_CONCURRENCY", 0); n > 0 {
		return n
	}
	return Parallel()
}

// Batch is the number of texts per embedding request.
func Batch() int {
	return envInt("DALI_EMBED_BATCH", defaultBatch)
}
