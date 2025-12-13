package main

import (
	"errors"
	"flag"
	"strings"
)

// ParseFlagsFlexible parses flags even if they appear after positional arguments.
// It reorders arguments such that all flags come before positional arguments,
// then calls fs.Parse.
// Note: This relies on fs having all flags defined before calling this function.
func ParseFlagsFlexible(fs *flag.FlagSet, args []string) error {
	var flagArgs []string
	var posArgs []string

	// We need to identify which flags are boolean to know if they consume an argument.
	boolFlags := make(map[string]bool)
	fs.VisitAll(func(f *flag.Flag) {
		if isBoolFlag(f.Value) {
			boolFlags[f.Name] = true
		}
	})

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) > 0 && (arg[0] == '-' || arg[0] == '/') {
			// Check if it's a known flag
			isFlag := false
			name := arg[1:]
			if len(name) > 0 && name[0] == '-' {
				name = name[1:]
			}
			// Handle --flag=value
			if strings.Contains(name, "=") {
				parts := strings.SplitN(name, "=", 2)
				name = parts[0]
			}

			if fs.Lookup(name) != nil {
				isFlag = true
			}

			if isFlag {
				flagArgs = append(flagArgs, arg)
				// If it's a bool flag (and not using =value syntax), no arg consumed.
				// If it's a non-bool flag (and not using =value syntax), next arg is consumed.
				if !strings.Contains(arg, "=") && !boolFlags[name] {
					if i+1 < len(args) {
						flagArgs = append(flagArgs, args[i+1])
						i++
					} else {
						// Flag requires argument but none found
						return errors.New("Flag needs an argument: " + arg)
					}
				}
				continue
			}
		}
		// If not a flag, or unknown flag (treated as positional arg here), add to posArgs
		posArgs = append(posArgs, arg)
	}

	// Reconstruct args: flags first, then positionals
	newArgs := append(flagArgs, posArgs...)
	return fs.Parse(newArgs)
}

type boolFlag interface {
	IsBoolFlag() bool
}

func isBoolFlag(v flag.Value) bool {
	if b, ok := v.(boolFlag); ok {
		return b.IsBoolFlag()
	}
	return false
}
