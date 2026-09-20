package query

import "testing"

func TestSuggestXPath(t *testing.T) {
	svc := openDSLService(t)
	defer svc.Close()

	sug, err := svc.SuggestXPath("//FRAME[@name='card']", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSuggest(sug, "Card") {
		t.Fatalf("SuggestXPath card = %+v, want Card", sug)
	}

	sug, err = svc.SuggestXPath("//TEXT[@name~'Pric']", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSuggest(sug, "Price") {
		t.Fatalf("SuggestXPath Pric = %+v, want Price", sug)
	}

	sug, err = svc.SuggestXPath("//FRAME", 5)
	if err != nil {
		t.Fatal(err)
	}
	if sug == nil {
		t.Fatalf("SuggestXPath //FRAME = nil, want non-nil")
	}

	sug, err = svc.SuggestXPath("/not valid[", 5)
	if err != nil {
		t.Fatal(err)
	}
	if sug != nil {
		t.Fatalf("SuggestXPath of unparsable expr = %+v, want nil", sug)
	}
}

func hasSuggest(sugs []Suggest, name string) bool {
	for _, s := range sugs {
		if s.Name == name {
			return true
		}
	}
	return false
}
