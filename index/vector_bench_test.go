package index

import (
	"fmt"
	"math/rand"
	"testing"
)

func randomVectors(n, dim int) []Vector {
	rng := rand.New(rand.NewSource(1))
	out := make([]Vector, n)
	for i := range out {
		v := make([]float32, dim)
		for j := range v {
			v[j] = rng.Float32()*2 - 1
		}
		out[i] = Vector{NodeID: fmt.Sprintf("n%d", i), Embedding: v}
	}
	return out
}

func BenchmarkSearchVectors(b *testing.B) {
	const dim = 1024
	query := randomVectors(1, dim)[0].Embedding
	for _, size := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			db, err := Open(":memory:")
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()
			if err := db.CreateVec(dim); err != nil {
				b.Fatal(err)
			}
			if err := db.UpsertVectors(randomVectors(size, dim)); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := db.SearchVectors(query, 10); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWriteEmbeddings(b *testing.B) {
	const dim = 768
	const batch = 64
	db, err := Open(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := db.CreateVec(dim); err != nil {
		b.Fatal(err)
	}
	vs := randomVectors(batch, dim)
	es := make([]Embedding, batch)
	for i := range es {
		es[i] = Embedding{NodeID: vs[i].NodeID, Model: "m", Dimensions: dim, ContentHash: "h", GeneratorVersion: 1}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.WriteEmbeddings(vs, es); err != nil {
			b.Fatal(err)
		}
	}
}
