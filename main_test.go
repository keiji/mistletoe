package main

import (
	"reflect"
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Config
		wantErr bool
	}{
		{
			name: "Valid JSON",
			input: `{
				"repositories": [
					{
						"repo": "user/repo",
						"branch": "main",
						"labels": ["bug", "feature"]
					}
				]
			}`,
			want: &Config{
				Repositories: []Repository{
					{
						Repo:   "user/repo",
						Branch: "main",
						Labels: []string{"bug", "feature"},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   `{ "repositories": [ ... invalid ... ] }`,
			want:    nil,
			wantErr: true,
		},
		{
			name:  "Empty JSON Object",
			input: `{}`,
			want: &Config{
				Repositories: nil, // or empty slice depending on json unmarshal behavior, typically nil if missing
			},
			wantErr: false,
		},
		{
			name: "Missing Fields",
			input: `{
				"repositories": [
					{
						"repo": "user/repo"
					}
				]
			}`,
			want: &Config{
				Repositories: []Repository{
					{
						Repo:   "user/repo",
						Branch: "",
						Labels: nil,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfig([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
