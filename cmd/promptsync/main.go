package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/kgilich/promptsync/internal/promptsync"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "promptsync:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "init":
		flags := flag.NewFlagSet("init", flag.ContinueOnError)
		root := flags.String("dir", ".", "directory to initialize")
		tools := flags.String("tools", strings.Join(promptsync.DefaultTools, ","), "comma-separated tool presets to generate (see 'promptsync presets')")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		if err := promptsync.InitWithTools(*root, splitList(*tools)); err != nil {
			return err
		}
		fmt.Printf("Initialized PromptSync in %s\n", *root)
		return nil
	case "import":
		flags := flag.NewFlagSet("import", flag.ContinueOnError)
		root := flags.String("dir", ".", "project directory to import from")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		result, err := promptsync.Import(*root)
		if err != nil {
			return err
		}
		for _, path := range result.Imported {
			fmt.Println("Imported", path)
		}
		fmt.Printf("Created promptsync.yaml and prompts/master.md for: %s\n", strings.Join(result.Tools, ", "))
		if result.Differing {
			fmt.Println("Warning: the files had different content. master.md keeps each version as a section and every tool will receive all of them; trim tool-specific parts before syncing.")
		}
		fmt.Println("Review prompts/master.md, then run 'promptsync sync --force' to replace the imported files with generated ones.")
		for _, path := range result.Leftover {
			fmt.Printf("Note: %s is a legacy file that PromptSync will not regenerate; delete it after syncing to avoid duplicate rules.\n", path)
		}
		return nil
	case "presets":
		if len(args) != 1 {
			return usageError()
		}
		for _, preset := range promptsync.Presets() {
			fmt.Printf("%-9s %-38s %s\n", preset.Name, preset.Path, preset.Tool)
		}
		return nil
	case "sync":
		flags := flag.NewFlagSet("sync", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		check := flags.Bool("check", false, "check generated targets without writing files")
		force := flags.Bool("force", false, "overwrite modified targets")
		watch := flags.Bool("watch", false, "re-sync automatically when the source, rules, or config change")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		if *watch {
			if *check {
				return fmt.Errorf("--watch and --check cannot be used together")
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			fmt.Println("Watching for changes; press Ctrl+C to stop.")
			return promptsync.Watch(ctx, *config, promptsync.SyncOptions{Force: *force}, 500*time.Millisecond, func(event promptsync.WatchEvent) {
				if event.Err != nil {
					fmt.Fprintln(os.Stderr, "promptsync:", event.Err)
					return
				}
				for _, path := range event.Written {
					fmt.Println("Wrote", path)
				}
			})
		}
		if *check && *force {
			return fmt.Errorf("--check and --force cannot be used together")
		}
		if *check {
			statuses, err := promptsync.Inspect(*config)
			if err != nil {
				return err
			}
			printStatuses(statuses)
			for _, status := range statuses {
				if status.State != promptsync.TargetCurrent {
					return fmt.Errorf("generated targets are out of date; run 'promptsync sync'")
				}
			}
			return nil
		}
		written, err := promptsync.SyncWithOptions(*config, promptsync.SyncOptions{Force: *force})
		if err != nil {
			return err
		}
		for _, path := range written {
			fmt.Println("Wrote", path)
		}
		return nil
	case "update":
		flags := flag.NewFlagSet("update", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		revision, err := promptsync.Update(*config)
		if err != nil {
			return err
		}
		fmt.Printf("Updated library to %s\n", revision)
		return nil
	case "install":
		flags := flag.NewFlagSet("install", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		revision, err := promptsync.Install(*config)
		if err != nil {
			return err
		}
		fmt.Printf("Installed library at %s\n", revision)
		return nil
	case "status":
		flags := flag.NewFlagSet("status", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		statuses, err := promptsync.Inspect(*config)
		if err != nil {
			return err
		}
		printStatuses(statuses)
		return nil
	case "help", "-h", "--help":
		fmt.Print("Usage: promptsync <init|import|presets|update|install|status|sync> [options]\n\nCommands:\n  init [-tools a,b]  Create a starter library and configuration\n  import             Adopt existing CLAUDE.md, AGENTS.md, .cursorrules, ... as the source\n  presets            List built-in tool presets\n  update             Fetch the configured remote prompt library and advance the lock\n  install            Fetch the exact library revision from the lockfile\n  status             Show whether configured targets are current\n  sync               Render the library into configured target files\n  sync --check       Check target freshness without writing files\n  sync --force       Overwrite modified targets\n  sync --watch       Re-sync automatically when inputs change\n")
		return nil
	default:
		return usageError()
	}
}

func printStatuses(statuses []promptsync.TargetStatus) {
	for _, status := range statuses {
		fmt.Printf("%-8s %s\n", status.State, status.Path)
	}
}

func usageError() error {
	return fmt.Errorf("usage: promptsync <init|import|presets|update|install|status|sync> [options] (run 'promptsync help' for details)")
}

func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
