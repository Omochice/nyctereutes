package gitlab

import "testing"

const (
	cfgBot  = "cfg-bot"
	cfgRepo = "g/cfg"
)

func TestResolveAuthors(t *testing.T) {
	tests := []struct {
		name      string
		author    *string
		cfgAuthor string
		want      string
	}{
		{name: "default when unset", author: nil, cfgAuthor: "", want: DefaultAuthor},
		{name: "flag wins", author: new("flag-bot"), cfgAuthor: cfgBot, want: "flag-bot"},
		{name: "config when flag unset", author: nil, cfgAuthor: cfgBot, want: cfgBot},
		{name: "empty flag is explicit", author: new(""), cfgAuthor: cfgBot, want: ""},
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
		group, repos := ResolveScope(new("g/x, g/y"), nil, []string{cfgRepo})
		if group != "" {
			t.Errorf("group = %q, want empty", group)
		}
		if len(repos) != 2 || repos[0] != "g/x" || repos[1] != "g/y" {
			t.Errorf("repos = %v, want [g/x g/y]", repos)
		}
	})

	t.Run("config repo when flag unset", func(t *testing.T) {
		_, repos := ResolveScope(nil, nil, []string{cfgRepo})
		if len(repos) != 1 || repos[0] != cfgRepo {
			t.Errorf("repos = %v, want [%s]", repos, cfgRepo)
		}
	})

	t.Run("group-path flag is passed through", func(t *testing.T) {
		group, _ := ResolveScope(nil, new("grp/sub"), nil)
		if group != "grp/sub" {
			t.Errorf("group = %q, want %q", group, "grp/sub")
		}
	})
}
