package main

import (
	"flag"
	"fmt"
	"os"

	"citeoai/internal/config"
	"citeoai/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "version", "-v", "--version":
		fmt.Println(version.Formatted())
	case "doctor":
		runDoctor(args)
	case "init":
		runInit(args)
	case "run":
		runAgent(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", command)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println("Usage: citeoai <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init       Initialize configuration and directory structure")
	fmt.Println("  doctor     Run preflight diagnostic checks")
	fmt.Println("  run        Execute a single prompt or task")
	fmt.Println("  tui        Launch the interactive terminal UI")
	fmt.Println("  session    Manage conversation sessions")
	fmt.Println("  skill      Manage markdown skills")
	fmt.Println("  memory     Manage persistent memory journal")
	fmt.Println("  stats      Show FinOps token ledger and cost metrics")
	fmt.Println("  version    Display build and version information")
	fmt.Println()
}

func runDoctor(args []string) {
	fmt.Println("CITEOAI Diagnostics")
	fmt.Println("===================")

	cfgPath := config.DefaultConfigPath()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		fmt.Printf("[FAIL] Config: %v\n", err)
	} else {
		fmt.Printf("[OK]   Config loaded (%s)\n", cfgPath)
		fmt.Printf("[OK]   Provider: %s\n", cfg.Provider)
		fmt.Printf("[OK]   Model: %s\n", cfg.ModelName)
		if cfg.APIKey != "" {
			fmt.Println("[OK]   API Key configured")
		} else {
			fmt.Println("[WARN] API Key missing")
		}
	}
}

func runInit(args []string) {
	cfgPath := config.DefaultConfigPath()
	cfg := config.DefaultConfig()
	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Initialized configuration at %s\n", cfgPath)
}

func runAgent(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	modelFlag := fs.String("model", "", "model override")
	providerFlag := fs.String("provider", "", "provider override")
	_ = fs.Parse(args)

	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	if *modelFlag != "" {
		cfg.ModelName = *modelFlag
	}
	if *providerFlag != "" {
		cfg.Provider = *providerFlag
	}

	remainingArgs := fs.Args()
	if len(remainingArgs) == 0 {
		fmt.Fprintln(os.Stderr, "error: prompt required for run command")
		os.Exit(2)
	}

	prompt := remainingArgs[0]
	fmt.Printf("CITEOAI executing: %s (model: %s)\n", prompt, cfg.ModelName)
}
