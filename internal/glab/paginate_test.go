package glab

import "testing"

func TestDecodePagesSplitsBackToBackArrays(t *testing.T) {
	pages, err := DecodePages[int]([]byte("[1,2]\n[3]\n[]\n"))
	if err != nil {
		t.Fatalf("DecodePages() error = %v", err)
	}
	if len(pages) != 3 || len(pages[0]) != 2 || pages[1][0] != 3 || pages[2] == nil {
		t.Errorf("pages = %v, want [[1 2] [3] []]", pages)
	}
}

func TestDecodePagesKeepsANullPageDistinctFromAnEmptyOne(t *testing.T) {
	pages, err := DecodePages[int]([]byte("null"))
	if err != nil {
		t.Fatalf("DecodePages() error = %v", err)
	}
	if len(pages) != 1 || pages[0] != nil {
		t.Errorf("pages = %v, want one nil page", pages)
	}
}

func TestDecodePagesReadsNoPageFromBlankOutput(t *testing.T) {
	pages, err := DecodePages[int]([]byte("  \n"))
	if err != nil {
		t.Fatalf("DecodePages() error = %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("pages = %v, want none", pages)
	}
}

func TestDecodePagesRejectsMalformedOutput(t *testing.T) {
	if _, err := DecodePages[int]([]byte("[1,")); err == nil {
		t.Error("DecodePages() error = nil, want a decode error")
	}
}
