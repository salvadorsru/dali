package index

import "testing"

func TestSearchVectorsWithoutVecDegrades(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	n, err := db.SearchVectors([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("search without vec index must not fail: %v", err)
	}
	if len(n) != 0 {
		t.Fatalf("expected no neighbors, got %+v", n)
	}
}

func TestVec0(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if !db.VecAvailable() {
		t.Fatal("sqlite-vec not available")
	}
	if _, err := db.VecVersion(); err != nil {
		t.Fatal(err)
	}

	if err := db.CreateVec(3); err != nil {
		t.Fatalf("create vec0 with TEXT primary key: %v", err)
	}
	err = db.UpsertVectors([]Vector{
		{NodeID: "1:1", Embedding: []float32{1, 0, 0}},
		{NodeID: "1:2", Embedding: []float32{0, 1, 0}},
		{NodeID: "1:3", Embedding: []float32{0, 0, 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	n, err := db.SearchVectors([]float32{1, 0, 0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(n) != 1 || n[0].NodeID != "1:1" {
		t.Fatalf("neighbors = %+v", n)
	}

	if err := db.UpsertVectors([]Vector{{NodeID: "1:1", Embedding: []float32{0, 1, 0}}}); err != nil {
		t.Fatal(err)
	}
	n, err = db.SearchVectors([]float32{0, 1, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(n) != 2 {
		t.Fatalf("after upsert neighbors = %+v", n)
	}
	for _, nb := range n {
		if nb.Distance > 1e-6 {
			t.Fatalf("expected distance 0 after upsert, got %+v", n)
		}
	}

	if err := db.DeleteVectors([]string{"1:2"}); err != nil {
		t.Fatal(err)
	}
	count, err := db.CountVectors()
	if err != nil || count != 2 {
		t.Fatalf("count = %d, err %v", count, err)
	}
}
