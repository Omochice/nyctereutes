package glab

import "testing"

func TestEncodePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"single slash", "group/project", "group%2Fproject"},
		{"multiple slashes", "group/sub/project", "group%2Fsub%2Fproject"},
		{"no slash", "project", "project"},
		{"space is percent-encoded", "a b", "a%20b"},
		{"slash and space combined", "a/b c", "a%2Fb%20c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncodePath(tt.in); got != tt.want {
				t.Errorf("EncodePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
