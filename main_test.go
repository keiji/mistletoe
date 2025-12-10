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

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantConfigFile string
		wantSubcmdName string
		wantSubcmdArgs []string
		wantErr        bool
	}{
		{
			name:           "Init command with file flag at end",
			args:           []string{"gitc", "init", ".", "--file", "repos.json"},
			wantConfigFile: "",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "--file", "repos.json"},
			wantErr:        false,
		},
		{
			name:           "Init command with short file flag",
			args:           []string{"gitc", "init", ".", "-f", "repos.json"},
			wantConfigFile: "",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "-f", "repos.json"},
			wantErr:        false,
		},
		{
			name:           "File flag at beginning",
			args:           []string{"gitc", "--file", "repos.json", "init", "."},
			wantConfigFile: "repos.json",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"."},
			wantErr:        false,
		},
		{
			name:           "File flag with equal sign",
			args:           []string{"gitc", "--file=repos.json", "init", "."},
			wantConfigFile: "repos.json",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"."},
			wantErr:        false,
		},
		{
			name:           "Short file flag with equal sign",
			args:           []string{"gitc", "-f=repos.json", "init", "."},
			wantConfigFile: "repos.json",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"."},
			wantErr:        false,
		},
		{
			name:           "No command (legacy)",
			args:           []string{"gitc", "--file", "repos.json"},
			wantConfigFile: "repos.json",
			wantSubcmdName: "",
			wantSubcmdArgs: nil,
			wantErr:        false,
		},
		{
			name:           "Missing file argument value after subcommand",
			args:           []string{"gitc", "init", ".", "--file"},
			wantConfigFile: "",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "--file"},
			wantErr:        false,
		},
		{
			name:           "Mixed flags and args",
			args:           []string{"gitc", "command", "-f", "conf.json", "arg1", "--flag2"},
			wantConfigFile: "",
			wantSubcmdName: "command",
			wantSubcmdArgs: []string{"-f", "conf.json", "arg1", "--flag2"},
			wantErr:        false,
		},
		{
			name:           "Subcommand flag treated as argument",
			args:           []string{"gitc", "init", "-f", "repos.json"},
			wantConfigFile: "",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"-f", "repos.json"},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConfigFile, gotSubcmdName, gotSubcmdArgs, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotConfigFile != tt.wantConfigFile {
				t.Errorf("parseArgs() configFile = %v, want %v", gotConfigFile, tt.wantConfigFile)
			}
			if gotSubcmdName != tt.wantSubcmdName {
				t.Errorf("parseArgs() subcmdName = %v, want %v", gotSubcmdName, tt.wantSubcmdName)
			}
			if !reflect.DeepEqual(gotSubcmdArgs, tt.wantSubcmdArgs) {
				// Handle nil vs empty slice if necessary, but DeepEqual handles it reasonably well usually.
				// However, if we append to nil slice, it becomes non-nil empty slice in some cases?
				// In my implementation: var subcmdArgs []string. If nothing appended, it is nil.
				t.Errorf("parseArgs() subcmdArgs = %v, want %v", gotSubcmdArgs, tt.wantSubcmdArgs)
			}
		})
	}
}
