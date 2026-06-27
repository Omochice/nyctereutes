package gitlab

import "testing"

func strptr(s string) *string { return &s }

func TestResolveAuthors(t *testing.T) {
	tests := []struct {
		name      string
		author    *string
		cfgAuthor string
		want      string
	}{
		{name: "default when unset", author: nil, cfgAuthor: "", want: DefaultAuthor},
		{name: "flag wins", author: strptr("flag-bot"), cfgAuthor: "cfg-bot", want: "flag-bot"},
		{name: "config when flag unset", author: nil, cfgAuthor: "cfg-bot", want: "cfg-bot"},
		{name: "empty flag is explicit", author: strptr(""), cfgAuthor: "cfg-bot", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAuthors(tt.author, tt.cfgAuthor)
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("ResolveAuthors(%v, %q) = %v, want [%q]", tt.author, tt.cfgAuthor, got, tt.want)
			}
		})
	}
}

func TestResolveScope(t *testing.T) {
	t.Run("flag repo wins over config", func(t *testing.T) {
		group, repos := ResolveScope(strptr("g/x, g/y"), nil, []string{"g/cfg"})
		if group != "" {
			t.Errorf("group = %q, want empty", group)
		}
		if len(repos) != 2 || repos[0] != "g/x" || repos[1] != "g/y" {
			t.Errorf("repos = %v, want [g/x g/y]", repos)
		}
	})

	t.Run("config repo when flag unset", func(t *testing.T) {
		_, repos := ResolveScope(nil, nil, []string{"g/cfg"})
		if len(repos) != 1 || repos[0] != "g/cfg" {
			t.Errorf("repos = %v, want [g/cfg]", repos)
		}
	})

	t.Run("group-path flag is passed through", func(t *testing.T) {
		group, _ := ResolveScope(nil, strptr("grp/sub"), nil)
		if group != "grp/sub" {
			t.Errorf("group = %q, want %q", group, "grp/sub")
		}
	})
}
