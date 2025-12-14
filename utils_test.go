package main

import (
	"strings"
	"testing"
)

func TestResolveCommonValues(t *testing.T) {
	tests := []struct {
		name         string
		fLong        string
		fShort       string
		pVal         int
		pValShort    int
		wantConfig   string
		wantParallel int
		wantErr      bool
	}{
		{
			name:         "Defaults",
			fLong:        "",
			fShort:       "",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Config from Long Flag",
			fLong:        "config.json",
			fShort:       "",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "config.json",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Config from Short Flag",
			fLong:        "",
			fShort:       "short.json",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "short.json",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Config Long Priority",
			fLong:        "long.json",
			fShort:       "short.json",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "long.json",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Parallel from Long Flag",
			fLong:        "",
			fShort:       "",
			pVal:         4,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: 4,
			wantErr:      false,
		},
		{
			name:         "Parallel from Short Flag",
			fLong:        "",
			fShort:       "",
			pVal:         DefaultParallel,
			pValShort:    8,
			wantConfig:   "",
			wantParallel: 8,
			wantErr:      false,
		},
		{
			name:         "Parallel Long Priority",
			fLong:        "",
			fShort:       "",
			pVal:         4,
			pValShort:    8,
			wantConfig:   "",
			wantParallel: 4,
			wantErr:      false,
		},
		{
			name:         "Parallel Too Low",
			fLong:        "",
			fShort:       "",
			pVal:         MinParallel - 1,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: 0,
			wantErr:      true,
		},
		{
			name:         "Parallel Too High",
			fLong:        "",
			fShort:       "",
			pVal:         MaxParallel + 1,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: 0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConfig, gotParallel, err := ResolveCommonValues(tt.fLong, tt.fShort, tt.pVal, tt.pValShort)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveCommonValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotConfig != tt.wantConfig {
					t.Errorf("ResolveCommonValues() config = %v, want %v", gotConfig, tt.wantConfig)
				}
				if gotParallel != tt.wantParallel {
					t.Errorf("ResolveCommonValues() parallel = %v, want %v", gotParallel, tt.wantParallel)
				}
			}
		})
	}
}

func TestRunGit(t *testing.T) {
	// Simple test to ensure RunGit calls exec correctly
	out, err := RunGit("", "git", "--version")
	if err != nil {
		t.Fatalf("RunGit failed: %v", err)
	}
	if !strings.HasPrefix(out, "git version") {
		t.Errorf("Expected output starting with 'git version', got %q", out)
	}
}
