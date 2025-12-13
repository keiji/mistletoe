package main

import (
	"reflect"
	"testing"
)

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
			args:           []string{"mstl", "init", ".", "--file", "repos.json"},
			wantConfigFile: "",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "--file", "repos.json"},
			wantErr:        false,
		},
		{
			name:           "Init command with short file flag",
			args:           []string{"mstl", "init", ".", "-f", "repos.json"},
			wantConfigFile: "",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "-f", "repos.json"},
			wantErr:        false,
		},
		{
			name:           "File flag at beginning",
			args:           []string{"mstl", "--file", "repos.json", "init", "."},
			wantConfigFile: "repos.json",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"."},
			wantErr:        false,
		},
		{
			name:           "File flag with equal sign",
			args:           []string{"mstl", "--file=repos.json", "init", "."},
			wantConfigFile: "repos.json",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"."},
			wantErr:        false,
		},
		{
			name:           "Short file flag with equal sign",
			args:           []string{"mstl", "-f=repos.json", "init", "."},
			wantConfigFile: "repos.json",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{"."},
			wantErr:        false,
		},
		{
			name:           "No command (legacy)",
			args:           []string{"mstl", "--file", "repos.json"},
			wantConfigFile: "repos.json",
			wantSubcmdName: "",
			wantSubcmdArgs: nil,
			wantErr:        false,
		},
		{
			name:           "Missing file argument value after subcommand",
			args:           []string{"mstl", "init", ".", "--file"},
			wantConfigFile: "",
			wantSubcmdName: "init",
			wantSubcmdArgs: []string{".", "--file"},
			wantErr:        false,
		},
		{
			name:           "Mixed flags and args",
			args:           []string{"mstl", "command", "-f", "conf.json", "arg1", "--flag2"},
			wantConfigFile: "",
			wantSubcmdName: "command",
			wantSubcmdArgs: []string{"-f", "conf.json", "arg1", "--flag2"},
			wantErr:        false,
		},
		{
			name:           "Subcommand flag treated as argument",
			args:           []string{"mstl", "init", "-f", "repos.json"},
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
				t.Errorf("parseArgs() subcmdArgs = %v, want %v", gotSubcmdArgs, tt.wantSubcmdArgs)
			}
		})
	}
}
