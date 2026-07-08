package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/Ret2Hell/lazyjira/internal/app"
	"github.com/Ret2Hell/lazyjira/internal/config"
	"github.com/Ret2Hell/lazyjira/internal/service"
	"github.com/Ret2Hell/lazyjira/internal/version"
)

func main() {
	var (
		configPath  string
		forceSetup  bool
		showVersion bool
	)
	flag.StringVar(&configPath, "config", "", "path to config.yaml")
	flag.BoolVar(&forceSetup, "setup", false, "open setup even when config exists")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		info := version.Get()
		fmt.Printf("%s %s\ncommit: %s\nbuilt: %s\n", info.Name, info.Version, info.Commit, info.Built)
		return
	}

	cfg, resolvedPath, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	factory := service.Factory(service.NewJiraService)
	var svc service.Service
	initialStatus := ""
	if forceSetup {
		initialStatus = "Update setup"
	} else if cfg.IsConfigured() {
		svc, err = factory(cfg)
		if err != nil {
			initialStatus = err.Error()
		}
	}

	model := app.New(cfg, resolvedPath, svc, factory, initialStatus)
	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
