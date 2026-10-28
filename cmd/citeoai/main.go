package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"

	"citeoai/internal/agent"
	"citeoai/internal/config"
	"citeoai/internal/interfaces"
	"citeoai/internal/memory"
	"citeoai/internal/provider/gemini"
	"citeoai/internal/provider/openai"
	"citeoai/internal/sandbox"
	"citeoai/internal/skill"
	"citeoai/internal/tools"
	"citeoai/internal/tui"
	"citeoai/internal/version"
	"citeoai/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		cfg, err := config.LoadConfig("")
		if err == nil && isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) {
			runTUI(cfg)
			return
		}
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
	case "tui":
		cfg, err := config.LoadConfig("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
			os.Exit(1)
		}
		runTUI(cfg)
	case "serve":
		runServe(args)
	case "memory":
		runMemory(args)
	case "skill":
		runSkill(args)
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
	fmt.Println("  serve      Start HTTP and WebSocket server for remote control")
	fmt.Println("  session    Manage conversation sessions")
	fmt.Println("  skill      Manage markdown skills")
	fmt.Println("  memory     Manage persistent memory journal")
	fmt.Println("  stats      Show FinOps token ledger and cost metrics")
	fmt.Println("  version    Display build and version information")
	fmt.Println()
}

func buildProvider(cfg *config.Config) interfaces.Provider {
	switch strings.ToLower(cfg.Provider) {
	case "openai", "openai-compatible", "ollama":
		return openai.New(cfg.APIKey, cfg.ModelName, openai.WithBaseURL(cfg.BaseURL), openai.WithName(cfg.Provider))
	case "gemini":
		fallthrough
	default:
		return gemini.New(cfg.APIKey, cfg.ModelName, gemini.WithBaseURL(cfg.BaseURL))
	}
}

func buildOrchestrator(cfg *config.Config) (*agent.AgentOrchestrator, error) {
	prov := buildProvider(cfg)
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	sb := sandbox.NewSandbox(cwd, cfg.Sandbox.AllowedPaths, cfg.Sandbox.DeniedPaths)
	registry := tools.NewDefaultRegistry(sb)
	memStore, err := memory.Open("")
	if err != nil {
		return nil, fmt.Errorf("memory initialization failed: %w", err)
	}
	skills, _ := skill.DiscoverSkills(nil)
	orch := agent.NewOrchestrator(cfg, prov, registry, memStore, skills)

	agyPath := "/opt/data/home/.local/bin/agy"
	if strings.TrimSpace(cfg.APIKey) == "" {
		if _, statErr := os.Stat(agyPath); statErr == nil {
			orch.SetCLIRunner(agent.NewCLIRunner(agyPath, "/opt/data/home/agy-profiles/account1", "gemini-3.8-flash-high"))
		}
	}

	return orch, nil
}

func runTUI(cfg *config.Config) {
	orch, err := buildOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = tui.RunTUI(cfg, func(ctx context.Context, prompt string, onToken func(string), onLog func(string)) error {
		_, turnErr := orch.RunTurn(ctx, prompt, onToken, onLog)
		return turnErr
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	portFlag := fs.Int("port", 8088, "server port")
	distFlag := fs.String("dist", "/opt/data/home/ui-citeoai/dist", "static dist directory")
	configFlag := fs.String("config", "", "config file path")
	_ = fs.Parse(args)

	cfg, err := config.LoadConfig(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	orch, err := buildOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator error: %v\n", err)
		os.Exit(1)
	}

	srv := web.NewServer(web.ServerConfig{
		Port:    *portFlag,
		DistDir: *distFlag,
	}, orch)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	fmt.Printf("CITEOAI Web Server listening on port %d\n", *portFlag)
	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func runAgent(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	modelFlag := fs.String("model", "", "model override")
	providerFlag := fs.String("provider", "", "provider override")
	configFlag := fs.String("config", "", "config file path")
	_ = fs.Parse(args)

	cfg, err := config.LoadConfig(*configFlag)
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

	prompt := strings.Join(remainingArgs, " ")

	orch, err := buildOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	_, err = orch.RunTurn(ctx, prompt, func(token string) {
		os.Stdout.WriteString(token)
	}, func(entry string) {
	})

	os.Stdout.WriteString("\n")

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runDoctor(args []string) {
	fmt.Println("CITEOAI Diagnostics")
	fmt.Println("===================")

	fmt.Printf("[OK]   Go toolchain: %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

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

	memStore, err := memory.Open("")
	if err != nil {
		fmt.Printf("[FAIL] Memory: %v\n", err)
	} else {
		fmt.Printf("[OK]   Memory storage (%s, %d notes)\n", memStore.FilePath(), len(memStore.List()))
	}

	activeCfg := cfg
	if activeCfg == nil {
		d := config.DefaultConfig()
		activeCfg = &d
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	sb := sandbox.NewSandbox(cwd, activeCfg.Sandbox.AllowedPaths, activeCfg.Sandbox.DeniedPaths)
	fmt.Printf("[OK]   Sandbox status (root: %s, %d allowed, %d denied)\n", sb.WorkspaceRoot(), len(sb.AllowedPaths()), len(sb.DeniedPaths()))

	skills, err := skill.DiscoverSkills(nil)
	if err != nil {
		fmt.Printf("[WARN] Skills: %v\n", err)
	} else {
		fmt.Printf("[OK]   Discovered skills: %d\n", len(skills))
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

func runMemory(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: citeoai memory <list|add|forget> [arguments]")
		os.Exit(2)
	}

	memStore, err := memory.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open memory: %v\n", err)
		os.Exit(1)
	}

	sub := args[0]
	switch sub {
	case "list":
		notes := memStore.List()
		if len(notes) == 0 {
			fmt.Println("No memory notes found.")
			return
		}
		for _, n := range notes {
			tags := strings.Join(n.Tags, ", ")
			if tags != "" {
				tags = " [" + tags + "]"
			}
			fmt.Printf("%s (%s)%s: %s\n", n.ID, n.Category, tags, n.Content)
		}
	case "add":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			fmt.Fprintln(os.Stderr, "error: note content required")
			os.Exit(2)
		}
		note, err := memStore.AddNote("general", []string{"cli"}, args[1], "cli")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to add note: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Added memory note: %s\n", note.ID)
	case "forget":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			fmt.Fprintln(os.Stderr, "error: note id required")
			os.Exit(2)
		}
		id := strings.TrimSpace(args[1])
		if err := memStore.Forget(id); err != nil {
			fmt.Fprintf(os.Stderr, "failed to forget note: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Forgotten memory note: %s\n", id)
	default:
		fmt.Fprintf(os.Stderr, "unknown memory command: %s\n", sub)
		os.Exit(2)
	}
}

func runSkill(args []string) {
	if len(args) == 0 || args[0] != "list" {
		fmt.Println("Usage: citeoai skill list")
		os.Exit(2)
	}

	skills, err := skill.DiscoverSkills(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to discover skills: %v\n", err)
		os.Exit(1)
	}

	if len(skills) == 0 {
		fmt.Println("No skills found.")
		return
	}

	for _, s := range skills {
		trigs := strings.Join(s.Triggers, ", ")
		if trigs != "" {
			trigs = " (triggers: " + trigs + ")"
		}
		fmt.Printf("- %s: %s%s\n", s.Name, s.Description, trigs)
	}
}
