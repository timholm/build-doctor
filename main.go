package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/timholm/build-doctor/internal/config"
	"github.com/timholm/build-doctor/internal/doctor"
	"github.com/timholm/build-doctor/internal/registry"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "build-doctor",
		Short: "Automatic test failure fixer for claude-code-factory",
		Long:  "Reads failed build logs, sends errors to Claude Code CLI, applies fixes, retries until tests pass.",
	}

	root.AddCommand(fixCmd())
	root.AddCommand(statsCmd())
	root.AddCommand(serveCmd())

	return root
}

func fixCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Fix failed builds by sending test errors to Claude",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			reg, err := registry.Open(cfg.RegistryPath())
			if err != nil {
				return fmt.Errorf("opening registry: %w", err)
			}
			defer reg.Close()

			doc := doctor.New(cfg, reg)

			if name != "" {
				return doc.FixOne(name)
			}
			return doc.FixAll()
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Fix a specific repo by name")
	return cmd
}

func statsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show fix success rate",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			reg, err := registry.Open(cfg.RegistryPath())
			if err != nil {
				return fmt.Errorf("opening registry: %w", err)
			}
			defer reg.Close()

			doc := doctor.New(cfg, reg)
			return doc.PrintStats(os.Stdout)
		},
	}
}

func serveCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP API for monitoring",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			reg, err := registry.Open(cfg.RegistryPath())
			if err != nil {
				return fmt.Errorf("opening registry: %w", err)
			}
			defer reg.Close()

			doc := doctor.New(cfg, reg)
			return doc.Serve(addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "Listen address")
	return cmd
}
