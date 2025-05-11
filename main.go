package main

import (
	"fmt"
	"os"

	"go-wrk/config"
	"go-wrk/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var logFile *os.File
	var err error

	if os.Getenv("BENCH_DEBUG") != "" {
		logFile, err = tea.LogToFile("debug.log", "bench")
		if err != nil {
			fmt.Println("Couldn't open a file for logging:", err)
			os.Exit(1)
		}
		defer logFile.Close()
		fmt.Println("Debug logging enabled: debug.log")
	} else {
		fmt.Println("Debug logging disabled. Set BENCH_DEBUG=true to enable.")
	}

	configDir := config.GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Could not create config directory %s: %v\n", configDir, err)
		os.Exit(1) // Or return an error
	}

	testCollections, err := config.LoadTestCollections(configDir)
	if err != nil {
		if logFile != nil {
			fmt.Fprintf(logFile, "Error loading test collections: %v\n", err)
		}
		fmt.Printf("Error loading test collections: %v\n", err)
		os.Exit(1)
	}

	m := tui.NewModel(testCollections, logFile)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		if logFile != nil {
			fmt.Fprintf(logFile, "Error running program: %v\n", err)
		}
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
