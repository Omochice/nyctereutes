package tui

import "testing"

func TestPathShorten(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "single segment is returned unchanged",
			path: "project",
			want: "project",
		},
		{
			name: "all but the last segment are shortened to their first rune",
			path: "group/sub/project",
			want: "g/s/project",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathShorten(tt.path); got != tt.want {
				t.Errorf("pathShorten(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
