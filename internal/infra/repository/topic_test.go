package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

func declaringTopics(topics ...string) *manifest.Repository {
	return &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: ownerGroup, Name: nameProj},
		Spec:     manifest.RepositorySpec{Topics: topics},
	}
}

func carryingTopics(topics ...string) *CurrentState {
	return &CurrentState{rawProject: rawProject{Topics: topics}}
}

type topicSearch struct {
	held  []string
	calls []string
}

func (s *topicSearch) Run(_ context.Context, args ...string) ([]byte, error) {
	s.calls = append(s.calls, strings.Join(args, " "))
	names := make([]string, 0, len(s.held))
	for _, name := range s.held {
		names = append(names, fmt.Sprintf(`{"name":%q}`, name))
	}
	return []byte("[" + strings.Join(names, ",") + "]"), nil
}

func TestCheckTopicCasingReportsACasingGitLabDoesNotHold(t *testing.T) {
	search := &topicSearch{held: []string{"rust"}}

	err := NewClient(search).CheckTopicCasing(context.Background(),
		declaringTopics("Go", "nix", "CLI", "Rust"), carryingTopics("go", "cli", "nix"))

	if !errors.Is(err, errTopicCasing) {
		t.Fatalf("err = %v, want errTopicCasing", err)
	}
	for _, want := range []string{"group/proj", `"Go"`, `"go"`, `"CLI"`, `"cli"`, `"Rust"`, `"rust"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err missing %s\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "nix") {
		t.Errorf("err names a topic whose casing matches\n%v", err)
	}
}

func TestCheckTopicCasingAcceptsWhatAProjectWriteCanReach(t *testing.T) {
	tests := []struct {
		name         string
		desired      *manifest.Repository
		carried      []string
		organization []string
	}{
		{"the casing the project carries", declaringTopics("go", "CLI"), []string{"CLI", "go"}, nil},
		{"the casing the organization holds", declaringTopics("go", "Rust"), []string{"go"}, []string{"Rust"}},
		{"a topic the organization does not hold", declaringTopics("go", "Rust"), []string{"go"}, nil},
		{"a topic being removed", declaringTopics("go"), []string{"go", "Rust"}, nil},
		{"an omitted list", &manifest.Repository{}, []string{"Go"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			search := &topicSearch{held: test.organization}

			err := NewClient(search).CheckTopicCasing(context.Background(),
				test.desired, carryingTopics(test.carried...))
			if err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

func TestCheckTopicCasingLooksUpOnlyTheTopicsTheProjectLacks(t *testing.T) {
	search := &topicSearch{}

	err := NewClient(search).CheckTopicCasing(context.Background(),
		declaringTopics("go", "C++", "nix"), carryingTopics("go"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if want := []string{
		"api topics?per_page=100&search=C%2B%2B",
		"api topics?per_page=100&search=nix",
	}; !slices.Equal(search.calls, want) {
		t.Errorf("calls = %v, want %v", search.calls, want)
	}
}

// The search also answers topics whose name or title merely contains the term.
func TestCheckTopicCasingComparesWithTheTopicNamedAlike(t *testing.T) {
	search := &topicSearch{held: []string{"C/C++", "c++"}}

	err := NewClient(search).CheckTopicCasing(context.Background(),
		declaringTopics("C++"), carryingTopics())

	if !errors.Is(err, errTopicCasing) || !strings.Contains(err.Error(), `held as "c++"`) {
		t.Errorf("err = %v, want the casing of c++ reported", err)
	}
}

func TestCheckTopicCasingMakesNoLookupForAProjectToBeCreated(t *testing.T) {
	search := &topicSearch{held: []string{"rust"}}

	err := NewClient(search).CheckTopicCasing(context.Background(),
		declaringTopics("Rust"), &CurrentState{IsNew: true})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if len(search.calls) != 0 {
		t.Errorf("calls = %v, want none", search.calls)
	}
}

func TestCheckTopicCasingRefusesALookupThatDidNotAnswer(t *testing.T) {
	errForbidden := errors.New("403 Forbidden")
	tests := []struct {
		name   string
		runner glab.RunnerFunc
		want   error
	}{
		{"a failed request", func(context.Context, ...string) ([]byte, error) {
			return nil, errForbidden
		}, errForbidden},
		{"a null page", func(context.Context, ...string) ([]byte, error) {
			return []byte(`null`), nil
		}, errNoTopicPage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := NewClient(test.runner).CheckTopicCasing(context.Background(),
				declaringTopics("Go", "Rust"), carryingTopics("go"))

			if !errors.Is(err, test.want) {
				t.Errorf("err = %v, want %v", err, test.want)
			}
			if !errors.Is(err, errTopicCasing) {
				t.Errorf("err = %v, want the carried mismatch still reported", err)
			}
			for _, want := range []string{"group/proj", `"Rust"`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err missing %s\n%v", want, err)
				}
			}
		})
	}
}
