package app

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseDependencies(t *testing.T) {
	validIDs := []string{"mstl-core", "mstl-ui", "mstl-api", "mstl-db", "other", "mstl1", "mstl2", "mstl3", "A", "B"}

	tests := []struct {
		name    string
		content string
		want    *DependencyGraph
		wantErr bool
	}{
		{
			name: "Basic dependencies",
			content: `
graph TD
  mstl-ui --> mstl-api
  mstl-api --> mstl-db
`,
			want: &DependencyGraph{
				Forward: map[string][]string{
					"mstl-ui":  {"mstl-api"},
					"mstl-api": {"mstl-db"},
				},
				Reverse: map[string][]string{
					"mstl-api": {"mstl-ui"},
					"mstl-db":  {"mstl-api"},
				},
			},
			wantErr: false,
		},
		{
			name: "Dotted arrows",
			content: `
mstl-ui -.-> mstl-api
`,
			want: &DependencyGraph{
				Forward: map[string][]string{
					"mstl-ui": {"mstl-api"},
				},
				Reverse: map[string][]string{
					"mstl-api": {"mstl-ui"},
				},
			},
			wantErr: false,
		},
		{
			name: "Mutual dependencies",
			content: `
mstl-core <--> mstl-ui
`,
			want: &DependencyGraph{
				Forward: map[string][]string{
					"mstl-core": {"mstl-ui"},
					"mstl-ui":   {"mstl-core"},
				},
				Reverse: map[string][]string{
					"mstl-ui":   {"mstl-core"},
					"mstl-core": {"mstl-ui"},
				},
			},
			wantErr: false,
		},
		{
			name: "With labels",
			content: `
mstl-ui["UI Component"] --> mstl-api[API]
mstl-api --> mstl-db
`,
			want: &DependencyGraph{
				Forward: map[string][]string{
					"mstl-ui":  {"mstl-api"},
					"mstl-api": {"mstl-db"},
				},
				Reverse: map[string][]string{
					"mstl-api": {"mstl-ui"},
					"mstl-db":  {"mstl-api"},
				},
			},
			wantErr: false,
		},
		{
			name: "Complex graph with labels and decorations",
			content: `
graph TD
    mstl1["frontend"] -- "entity" --> mstl2{backend}
    mstl2 -.-> mstl3("common")
    mstl1 ==> mstl3
`,
			want: &DependencyGraph{
				Forward: map[string][]string{
					"mstl1": {"mstl2", "mstl3"},
					"mstl2": {"mstl3"},
				},
				Reverse: map[string][]string{
					"mstl2": {"mstl1"},
					"mstl3": {"mstl1", "mstl2"},
				},
			},
			wantErr: false,
		},
		{
			name: "Ignore undirected arrows",
			content: `
mstl1 --o mstl2
mstl1 --x mstl2
mstl1 --- mstl2
`,
			want: &DependencyGraph{
				Forward: map[string][]string{},
				Reverse: map[string][]string{},
			},
			wantErr: false,
		},
		{
			name: "Labeled arrows variations",
			content: `
A -- label --> B
A -. label .-> B
A == label ==> B
`,
			want: &DependencyGraph{
				Forward: map[string][]string{
					"A": {"B"},
				},
				Reverse: map[string][]string{
					"B": {"A"},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid ID",
			content: `
mstl-ui --> unknown-repo
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid Left ID",
			content: `
unknown-repo --> mstl-api
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "Ignore unrelated lines",
			content: `
%% This is a comment
graph TD
mstl-ui --> mstl-api
subgraph foo
end
`,
			want: &DependencyGraph{
				Forward: map[string][]string{
					"mstl-ui": {"mstl-api"},
				},
				Reverse: map[string][]string{
					"mstl-api": {"mstl-ui"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDependencies(tt.content, validIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Normalize maps for comparison (nil vs empty map, order of slices)
				normalizeGraph(got)
				normalizeGraph(tt.want)

				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ParseDependencies() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func normalizeGraph(g *DependencyGraph) {
	if g == nil {
		return
	}
	if g.Forward == nil {
		g.Forward = make(map[string][]string)
	}
	if g.Reverse == nil {
		g.Reverse = make(map[string][]string)
	}
	for k, v := range g.Forward {
		// Remove duplicates before sort (ParseDependencies logic prevents duplicates but test manual construction might not)
		// But addDependency checks duplicates.
		// Just sort.
		sort.Strings(v)
		g.Forward[k] = v
	}
	for k, v := range g.Reverse {
		sort.Strings(v)
		g.Reverse[k] = v
	}
}
