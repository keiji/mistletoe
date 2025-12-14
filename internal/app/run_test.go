package app

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantSubcmdName string
		wantSubcmdArgs []string
		wantErr        bool
	}{
		{
			name:           "Init command with file flag at end",
			args:           []string{"mstl", "init", ".", "--file", "repos.json"},
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "--file", "repos.json"},
			wantErr:        false,
		},
		{
			name:           "Init command with short file flag",
			args:           []string{"mstl", "init", ".", "-f", "repos.json"},
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "-f", "repos.json"},
			wantErr:        false,
		},
		{
			name:           "File flag at beginning (treated as subcommand now)",
			args:           []string{"mstl", "--file", "repos.json", "init", "."},
			wantSubcmdName: "--file",
			wantSubcmdArgs: []string{"repos.json", "init", "."},
			wantErr:        false,
		},
		{
			name:           "File flag with equal sign (treated as subcommand)",
			args:           []string{"mstl", "--file=repos.json", "init", "."},
			wantSubcmdName: "--file=repos.json",
			wantSubcmdArgs: []string{"init", "."},
			wantErr:        false,
		},
		{
			name:           "Short file flag with equal sign (treated as subcommand)",
			args:           []string{"mstl", "-f=repos.json", "init", "."},
			wantSubcmdName: "-f=repos.json",
			wantSubcmdArgs: []string{"init", "."},
			wantErr:        false,
		},
		{
			name:           "No command (legacy) - empty args",
			args:           []string{"mstl"},
			wantSubcmdName: "",
			wantSubcmdArgs: nil,
			wantErr:        false,
		},
		{
			name:           "Missing file argument value after subcommand",
			args:           []string{"mstl", "init", ".", "--file"},
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "--file"},
			wantErr:        false,
		},
		{
			name:           "Mixed flags and args",
			args:           []string{"mstl", "command", "-f", "conf.json", "arg1", "--flag2"},
			wantSubcmdName: "command",
			wantSubcmdArgs: []string{"-f", "conf.json", "arg1", "--flag2"},
			wantErr:        false,
		},
		{
			name:           "Subcommand flag treated as argument",
			args:           []string{"mstl", "init", "-f", "repos.json"},
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"-f", "repos.json"},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSubcmdName, gotSubcmdArgs, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotSubcmdName != tt.wantSubcmdName {
				t.Errorf("parseArgs() subcmdName = %v, want %v", gotSubcmdName, tt.wantSubcmdName)
			}
			if !reflect.DeepEqual(gotSubcmdArgs, tt.wantSubcmdArgs) {
				t.Errorf("parseArgs() subcmdArgs = %v, want %v", gotSubcmdArgs, tt.wantSubcmdArgs)
			}
		})
	}
}
