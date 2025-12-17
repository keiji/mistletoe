package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func handlePrCheckout(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr checkout", flag.ExitOnError)
	var (
		uLong     string
		uShort    string
		pVal      int
		pValShort int
	)

	fs.StringVar(&uLong, "url", "", "Pull Request URL")
	fs.StringVar(&uShort, "u", "", "Pull Request URL (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	prURL := uLong
	if prURL == "" {
		prURL = uShort
	}

	if prURL == "" {
		fmt.Println("Error: Pull Request URL is required (-u or --url)")
		os.Exit(1)
	}

	parallel := pVal
	if pValShort != DefaultParallel {
		parallel = pValShort
	}

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 2. Fetch PR Body
	fmt.Printf("Fetching Pull Request information from %s...\n", prURL)
	cmd := execCommand(opts.GhPath, "pr", "view", prURL, "--json", "body", "-q", ".body")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error fetching PR body: %v\n", err)
		os.Exit(1)
	}
	prBody := string(out)

	// 3. Parse Mistletoe Block
	fmt.Println("Parsing Mistletoe block...")
	config, relatedJSON, err := ParseMistletoeBlock(prBody)
	if err != nil {
		fmt.Printf("Error parsing Mistletoe block: %v\n", err)
		os.Exit(1)
	}

	// (Optional) We read relatedJSON as per requirement, but currently don't use it for init logic.
	// We can display it or validate it if needed.
	if len(relatedJSON) > 0 {
		var rel map[string]interface{}
		if err := json.Unmarshal(relatedJSON, &rel); err != nil {
			fmt.Printf("Warning: related PR JSON is invalid: %v\n", err)
		}
	}

	// 4. Init / Checkout
	fmt.Println("Initializing repositories based on snapshot...")
	// The snapshot contains the target state. We treat it as the config.
	// PerformInit handles validation, cloning, and checking out.
	if err := PerformInit(*config.Repositories, opts.GitPath, parallel, 0); err != nil {
		fmt.Printf("Error during initialization: %v\n", err)
		// We continue to status even if some failed? Or exit?
		// Usually Init failure is critical.
		os.Exit(1)
	}

	// 5. Status
	fmt.Println("Verifying status...")
	spinner := NewSpinner()
	spinner.Start()
	rows := CollectStatus(config, parallel, opts.GitPath)
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath)
	spinner.Stop()

	RenderPrStatusTable(prRows)
}
