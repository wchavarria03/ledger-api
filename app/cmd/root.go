package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"ledger-api/app/internal/core"
)

type cliConfig struct {
	Verbose        bool
	OutputDir      string
	InputDir       string
	DryRun         bool
	SupabaseURL    string
	SupabaseKey    string
	SupabaseAnonKey string
	ServerAddr     string
	UserID         string
	AllowedOrigins string
}

var (
	cfg  = &cliConfig{}
	deps *core.Dependencies
)

var rootCmd = &cobra.Command{
	Use:   "ledger-api",
	Short: "Ingest bank statement PDFs, store transactions, and serve them via HTTP API",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(cfg.InputDir); os.IsNotExist(err) {
			return fmt.Errorf("input directory does not exist: %s", cfg.InputDir)
		}
		if err := os.MkdirAll(cfg.OutputDir, 0750); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		origins := []string{"*"}
		if cfg.AllowedOrigins != "" {
			origins = strings.Split(cfg.AllowedOrigins, ",")
		}

		var err error
		deps, err = core.NewDependencies(core.Config{
			SupabaseURL:     cfg.SupabaseURL,
			SupabaseKey:     cfg.SupabaseKey,
			SupabaseAnonKey: cfg.SupabaseAnonKey,
			ServerAddr:      cfg.ServerAddr,
			UserID:          cfg.UserID,
			AllowedOrigins:  origins,
		})
		if err != nil {
			return fmt.Errorf("initialising dependencies: %w", err)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&cfg.OutputDir, "output", "o", "data/output", "Output directory")
	rootCmd.PersistentFlags().StringVarP(&cfg.InputDir, "input-dir", "i", "data/input", "Input PDF directory")
	rootCmd.PersistentFlags().BoolVar(&cfg.DryRun, "dry-run", false, "Write to files instead of Supabase")
	rootCmd.PersistentFlags().StringVar(&cfg.SupabaseURL, "supabase-url", os.Getenv("SUPABASE_URL"), "Supabase project URL")
	rootCmd.PersistentFlags().StringVar(&cfg.SupabaseKey, "supabase-key", os.Getenv("SUPABASE_KEY"), "Supabase service role key")
	rootCmd.PersistentFlags().StringVar(&cfg.SupabaseAnonKey, "supabase-anon-key", os.Getenv("SUPABASE_ANON_KEY"), "Supabase anon key (for user-context requests)")
	defaultAddr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		defaultAddr = ":" + port
	}
	rootCmd.PersistentFlags().StringVar(&cfg.ServerAddr, "addr", defaultAddr, "HTTP server listen address")
	rootCmd.PersistentFlags().StringVar(&cfg.UserID, "user-id", os.Getenv("LEDGER_USER_ID"), "Supabase user ID to associate imported data with")
	rootCmd.PersistentFlags().StringVar(&cfg.AllowedOrigins, "cors-origins", os.Getenv("ALLOWED_ORIGINS"), "Comma-separated allowed CORS origins (default: *)")
}
