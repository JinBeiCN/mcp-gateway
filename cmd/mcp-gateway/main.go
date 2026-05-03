package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
	"github.com/JinBeiCN/mcp-gateway/internal/gateway"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	showVersion := flag.Bool("version", false, "show version and exit")
	validateConfig := flag.Bool("validate", false, "validate configuration and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("mcp-gateway %s\n", version)
		fmt.Printf("  build: %s\n", buildTime)
		fmt.Printf("  commit: %s\n", gitCommit)
		fmt.Printf("  go: %s\n", runtime.Version())
		return
	}

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Fatalf("config file not found: %s\nRun 'mcp-gateway --config <path>' or create config.yaml", *configPath)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if *validateConfig {
		fmt.Printf("Configuration valid.\n")
		fmt.Printf("  Server: %s v%s on %s\n", cfg.Server.Name, cfg.Server.Version, cfg.Server.ListenAddr)
		fmt.Printf("  Backends: %d configured\n", len(cfg.Backends))
		for _, b := range cfg.Backends {
			status := "enabled"
			if !b.Enabled {
				status = "disabled"
			}
			fmt.Printf("    - %s (%s): %s %v [%s]\n", b.Name, b.Description, b.Command, b.Args, status)
		}
		fmt.Printf("  Auth: %v\n", cfg.Security.Auth.Enabled)
		fmt.Printf("  Injection Detection: %v (block=%.1f, warn=%.1f)\n",
			cfg.Security.Injection.Enabled,
			cfg.Security.Injection.BlockThreshold,
			cfg.Security.Injection.WarnThreshold)
		fmt.Printf("  Audit Log: %s\n", cfg.Logging.AuditFile)
		return
	}

	log.Printf("[mcp-gateway] %s v%s starting", cfg.Server.Name, cfg.Server.Version)
	log.Printf("[mcp-gateway] config: %s", *configPath)

	gw, err := gateway.New(cfg)
	if err != nil {
		log.Fatalf("failed to create gateway: %v", err)
	}

	if err := gw.Start(); err != nil {
		log.Fatalf("gateway error: %v", err)
	}
}
