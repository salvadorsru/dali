package query

import (
	"reflect"
	"testing"
)

func TestDependents(t *testing.T) {
	svc := openDSLService(t)
	defer svc.Close()

	got, err := svc.Dependents("0:10")
	if err != nil {
		t.Fatal(err)
	}
	want := []Dependent{
		{ID: "0:2", Name: "Screen", Type: "FRAME", Places: 1},
		{ID: "0:12", Name: "Modal", Type: "FRAME", Places: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("Dependents = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Dependents[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestContainerOf(t *testing.T) {
	svc := openDSLService(t)
	defer svc.Close()

	n, err := svc.Node("0:11")
	if err != nil || n == nil {
		t.Fatal("CTA not found")
	}
	container, err := svc.containerOf(n)
	if err != nil {
		t.Fatal(err)
	}
	if container == nil || container.ID != "0:2" || container.Name != "Screen" {
		t.Fatalf("containerOf(CTA) = %+v, want Screen 0:2", container)
	}

	direct, err := svc.Node("0:13")
	if err != nil || direct == nil {
		t.Fatal("CTA2 not found")
	}
	container, err = svc.containerOf(direct)
	if err != nil {
		t.Fatal(err)
	}
	if container == nil || container.ID != "0:12" {
		t.Fatalf("containerOf(CTA2) = %+v, want Modal 0:12", container)
	}

	page, err := svc.Node("0:1")
	if err != nil || page == nil {
		t.Fatal("page not found")
	}
	container, err = svc.containerOf(page)
	if err != nil {
		t.Fatal(err)
	}
	if container != nil {
		t.Fatalf("containerOf(page) = %+v, want nil", container)
	}
}

func TestDependentsUnknown(t *testing.T) {
	svc := openDSLService(t)
	defer svc.Close()

	got, err := svc.Dependents("0:999")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []Dependent{}) {
		t.Fatalf("Dependents unknown = %+v, want empty", got)
	}
}
