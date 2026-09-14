package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/DoraTiger/doratiger-counter/internal/config"
	"github.com/DoraTiger/doratiger-counter/internal/data"
	"github.com/DoraTiger/doratiger-counter/internal/handler"
	appserver "github.com/DoraTiger/doratiger-counter/internal/server"
	"github.com/DoraTiger/doratiger-counter/internal/version"
)

var cfgFile string

func main() {
	rootCmd := &cobra.Command{
		Use:     "doratiger-counter",
		Short:   "轻量级页面访问计数器",
		Version: version.BuildVersion,
	}
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.toml", "配置文件路径")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 HTTP 服务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return runServer(cmd.Context(), cfg)
		},
	}
	initCmd := &cobra.Command{
		Use:   "init-config",
		Short: "生成默认配置文件",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.WriteConfig(cfgFile, config.DefaultConfig()); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Printf("配置文件已生成: %s\n", cfgFile)
			return nil
		},
	}

	rootCmd.AddCommand(serveCmd, initCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(parent context.Context, cfg *config.Config) (runErr error) {
	db, err := data.OpenSQLite(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, db.Close()) }()

	if err := data.MigrateForSite(db, cfg.Counter.SiteKey); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	siteKeys := map[string]struct{}{cfg.Counter.SiteKey: {}}
	for _, siteKey := range cfg.Counter.Sites {
		if siteKey != "" {
			siteKeys[siteKey] = struct{}{}
		}
	}
	repos := make(map[string]*data.CounterRepo, len(siteKeys))
	for siteKey := range siteKeys {
		repo, err := data.NewCounterRepo(db, siteKey)
		if err != nil {
			return fmt.Errorf("load counters for %q: %w", siteKey, err)
		}
		repos[siteKey] = repo
	}
	defer func() {
		for _, repo := range repos {
			runErr = errors.Join(runErr, repo.Stop())
		}
	}()
	counterHandler := handler.NewCounterHandler(repos, &cfg.Counter)
	httpServer := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      appserver.New(counterHandler, cfg),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	ctx, stopSignals := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.ListenAndServe() }()

	fmt.Printf("doratiger-counter %s listening on %s (db: sqlite)\n",
		version.BuildVersion, cfg.Server.Addr)

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
