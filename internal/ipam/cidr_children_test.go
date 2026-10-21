package ipam

import "testing"

func TestChildPrefixes24(t *testing.T) {
	parent, err := ParsePrefix("10.0.0.0/22")
	if err != nil {
		t.Fatal(err)
	}
	kids, err := ChildPrefixes(parent, 24)
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 4 {
		t.Fatalf("got %d want 4", len(kids))
	}
	if kids[0].String() != "10.0.0.0/24" || kids[3].String() != "10.0.3.0/24" {
		t.Fatalf("unexpected kids: %v %v", kids[0], kids[3])
	}
}

func TestChildPrefixesMax256(t *testing.T) {
	parent, err := ParsePrefix("10.0.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	kids, err := ChildPrefixes(parent, 24)
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 256 {
		t.Fatalf("got %d want 256", len(kids))
	}
}

func TestChildPrefixesTooMany(t *testing.T) {
	parent, err := ParsePrefix("10.0.0.0/15")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ChildPrefixes(parent, 24)
	if err == nil {
		t.Fatal("expected error for 512 children")
	}
}
