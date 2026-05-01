package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LFroesch/scout/internal/logger"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	rootFlag := flag.String("root", "", "restrict navigation to this directory (disables bookmarks outside it)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "scout — TUI file explorer with vim keys, search, preview, bookmarks\n\n")
		fmt.Fprintf(os.Stderr, "Usage: scout [flags]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("scout " + version)
		os.Exit(0)
	}

	// Resolve to absolute path
	var rootPath string
	if *rootFlag != "" {
		abs, err := filepath.Abs(*rootFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid --root path: %v\n", err)
			os.Exit(1)
		}
		rootPath = abs
	}

	// Initialize logger
	if err := logger.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logger: %v\n", err)
	}
	defer logger.Close()

	// SCOUT_LOG_LEVEL=debug|info|warn|error overrides the default Info level.
	switch strings.ToLower(os.Getenv("SCOUT_LOG_LEVEL")) {
	case "debug":
		logger.SetLevel(logger.LevelDebug)
	case "info", "":
		// default
	case "warn":
		logger.SetLevel(logger.LevelWarn)
	case "error":
		logger.SetLevel(logger.LevelError)
	}

	cwd, _ := os.Getwd()
	logger.Info("scout started: pid=%d cwd=%s root=%q", os.Getpid(), cwd, rootPath)

	m := initialModel(rootPath)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logger.Error("Program crashed: %v", err)
		log.Fatal(err)
	}
	logger.Info("scout exited cleanly")
}
