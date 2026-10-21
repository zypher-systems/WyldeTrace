package oui

import "testing"

func TestLookup(t *testing.T) {
	if got := Lookup("00:0c:29:ab:cd:ef"); got != "VMware" {
		t.Fatalf("got %q want VMware", got)
	}
	if got := Lookup(""); got != "" {
		t.Fatalf("empty mac should be empty vendor")
	}
}

func TestSuggest(t *testing.T) {
	s := SuggestDeviceType("00:0c:29:11:22:33", []int{22, 443}, nil)
	if s.Vendor != "VMware" || s.DeviceTypeName != "Virtual Machine" {
		t.Fatalf("unexpected suggestion: %+v", s)
	}
}
