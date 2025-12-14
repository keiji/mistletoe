package app

import (
	"flag"
	"reflect"
	"testing"
)

func TestParseFlagsFlexible(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedFlags map[string]string
		expectedArgs  []string
		expectError   bool
		setupFs       func(*flag.FlagSet)
	}{
		{
			name: "flags before args",
			args: []string{"-s", "value", "pos"},
			expectedFlags: map[string]string{
				"s": "value",
			},
			expectedArgs: []string{"pos"},
			setupFs: func(fs *flag.FlagSet) {
				fs.String("s", "", "string flag")
			},
		},
		{
			name: "flags after args",
			args: []string{"pos", "-s", "value"},
			expectedFlags: map[string]string{
				"s": "value",
			},
			expectedArgs: []string{"pos"},
			setupFs: func(fs *flag.FlagSet) {
				fs.String("s", "", "string flag")
			},
		},
		{
			name: "mixed flags and args",
			args: []string{"pos1", "-s", "value", "pos2"},
			expectedFlags: map[string]string{
				"s": "value",
			},
			expectedArgs: []string{"pos1", "pos2"},
			setupFs: func(fs *flag.FlagSet) {
				fs.String("s", "", "string flag")
			},
		},
		{
			name: "bool flag",
			args: []string{"pos", "-b"},
			expectedFlags: map[string]string{
				"b": "true",
			},
			expectedArgs: []string{"pos"},
			setupFs: func(fs *flag.FlagSet) {
				fs.Bool("b", false, "bool flag")
			},
		},
		{
			name: "bool flag with other flags",
			args: []string{"-b", "pos", "-s", "value"},
			expectedFlags: map[string]string{
				"b": "true",
				"s": "value",
			},
			expectedArgs: []string{"pos"},
			setupFs: func(fs *flag.FlagSet) {
				fs.Bool("b", false, "bool flag")
				fs.String("s", "", "string flag")
			},
		},
		{
			name:        "flag missing value at end",
			args:        []string{"pos", "-s"},
			expectError: true,
			setupFs: func(fs *flag.FlagSet) {
				fs.String("s", "", "string flag")
			},
		},
		{
			name:        "flag missing value in middle",
			args:        []string{"-s"}, // technically in middle if we consider empty rest? No, just missing value
			expectError: true,
			setupFs: func(fs *flag.FlagSet) {
				fs.String("s", "", "string flag")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			tt.setupFs(fs)

			err := ParseFlagsFlexible(fs, tt.args)
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFlagsFlexible failed: %v", err)
			}

			// Check flags
			fs.Visit(func(f *flag.Flag) {
				expectedVal, ok := tt.expectedFlags[f.Name]
				if !ok {
					t.Errorf("Unexpected flag set: %s", f.Name)
					return
				}
				if f.Value.String() != expectedVal {
					t.Errorf("Flag %s value mismatch: got %s, want %s", f.Name, f.Value.String(), expectedVal)
				}
				delete(tt.expectedFlags, f.Name)
			})

			if len(tt.expectedFlags) > 0 {
				t.Errorf("Expected flags not set: %v", tt.expectedFlags)
			}

			// Check args
			args := fs.Args()
			if !reflect.DeepEqual(args, tt.expectedArgs) {
				t.Errorf("Args mismatch: got %v, want %v", args, tt.expectedArgs)
			}
		})
	}
}
