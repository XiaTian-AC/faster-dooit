// Package main is the faster-dooit TUI entry point.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/XiaTian-AC/faster-dooit/internal/app"
	"github.com/XiaTian-AC/faster-dooit/internal/lua"
	"github.com/XiaTian-AC/faster-dooit/internal/store"
)

// version is the embedded version string; bumped by hand.
const version = "0.1.0"

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.printVersion {
		fmt.Println("faster-dooit -", version)
		return
	}
	if cfg.printConfig {
		fmt.Println(cfg.configPath)
		return
	}

	if err := os.MkdirAll(filepath.Dir(cfg.dbPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "faster-dooit: prepare data dir:", err)
		os.Exit(1)
	}

	st, err := store.New(cfg.dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "faster-dooit: open db:", err)
		os.Exit(1)
	}
	defer st.Close()

	// Evaluate config.lua. A missing file falls back to nil (built-in
	// defaults); an invalid config prints the file+line error and exits.
	var rt *lua.Runtime
	if _, statErr := os.Stat(cfg.configPath); statErr == nil {
		rt, err = lua.EvalFile(cfg.configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "faster-dooit: config error:", err)
			os.Exit(1)
		}
		defer rt.Close()
	}

	model := app.New(st, rt)
	if err := model.RefreshFromStore(); err != nil {
		fmt.Fprintln(os.Stderr, "faster-dooit: load db:", err)
		os.Exit(1)
	}
	// Skeleton entry: loads and prints the workspace count. The interactive
	// TUI (tea.NewProgram) is wired in a later milestone.
	fmt.Printf("faster-dooit skeleton loaded %d workspaces from %s\n", visibleTopLevel(model), cfg.dbPath)
}

type runConfig struct {
	dbPath       string
	configPath   string
	printVersion bool
	printConfig  bool
}

func parseFlags(args []string) (runConfig, error) {
	cfg := runConfig{
		dbPath:     defaultDBPath(),
		configPath: defaultConfigPath(),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-v", "--version":
			cfg.printVersion = true
		case "--config-loc":
			cfg.printConfig = true
		case "-c", "--config":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for %s", args[i])
			}
			i++
			cfg.configPath = args[i]
		case "--db":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("missing value for %s", args[i])
			}
			i++
			cfg.dbPath = args[i]
		default:
			return cfg, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return cfg, nil
}

func defaultDBPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "faster-dooit", "todo.db")
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "faster-dooit", "config.lua")
}

// visibleTopLevel is a small helper to display the skeleton-load message
// without exposing the internal app type.
func visibleTopLevel(m *app.Model) int {
	return len(m.VisibleWorkspaces())
}
