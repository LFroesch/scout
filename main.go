package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LFroesch/scout/internal/logger"
)

func main() {
	rootFlag := flag.String("root", "", "restrict navigation to this directory (disables bookmarks outside it)")
	flag.Parse()

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

	m := initialModel(rootPath)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logger.Error("Program crashed: %v", err)
		log.Fatal(err)
	}
}
