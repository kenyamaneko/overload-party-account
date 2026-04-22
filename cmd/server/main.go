package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	pubsubadapter "github.com/kenyamaneko/overload-party-account/internal/adapter/pubsub"
	"github.com/kenyamaneko/overload-party-account/internal/config"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	accountfirestore "github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("account fatal", "error", err)
		os.Exit(1)
	}
}

// setupLogger は LOG_MODE に応じてグローバル slog ロガーを初期化する。
// production は Cloud Logging 互換 JSON、local は人間向け TextHandler を使う。
func setupLogger(mode config.LogMode) error {
	switch mode {
	case config.LogModeProduction:
		slog.SetDefault(slog.New(newCloudLoggingHandler()).With("service", "account"))
	case config.LogModeLocal:
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		slog.SetDefault(slog.New(h).With("service", "account"))
	default:
		return fmt.Errorf("unexpected LOG_MODE: %s", mode)
	}
	return nil
}

// newCloudLoggingHandler は Cloud Logging 互換の JSON ハンドラを返す。
// slog のデフォルトフィールド名・値では Cloud Logging が認識しないため変換する。
func newCloudLoggingHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				a.Key = "severity"
				if level, ok := a.Value.Any().(slog.Level); ok {
					switch {
					case level >= slog.LevelError:
						a.Value = slog.StringValue("ERROR")
					case level >= slog.LevelWarn:
						a.Value = slog.StringValue("WARNING")
					case level >= slog.LevelInfo:
						a.Value = slog.StringValue("INFO")
					default:
						a.Value = slog.StringValue("DEBUG")
					}
				}
			}
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	})
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	if err := setupLogger(cfg.LogMode); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool new: %w", err)
	}
	defer pool.Close()

	fsClient, err := firestore.NewClient(ctx, cfg.FirestoreProjectID)
	if err != nil {
		return fmt.Errorf("firestore new client: %w", err)
	}
	defer func() {
		if cerr := fsClient.Close(); cerr != nil {
			slog.Warn("firestore client close", "error", cerr)
		}
	}()

	txManager := postgres.NewTxManager(pool)
	playerRepo := postgres.NewPlayerRepository(pool)
	userSettingsRepo := postgres.NewUserSettingsRepository(pool)
	gameConfigRepo := accountfirestore.NewGameConfigRepository(fsClient)
	factionRepo := postgres.NewFactionRepository(pool)
	eventRepo := postgres.NewProcessedEventRepository(pool)

	authSvc := service.NewAuthService(playerRepo, userSettingsRepo, txManager)
	playerSvc := service.NewPlayerService(playerRepo, gameConfigRepo, factionRepo, txManager)
	factionSvc := service.NewFactionService(playerRepo, factionRepo, eventRepo, txManager)
	settingsSvc := service.NewUserSettingsService(userSettingsRepo)

	authH := rest.NewAuthHandler(authSvc)
	playerH := rest.NewPlayerHandler(playerSvc)
	factionH := rest.NewFactionHandler(factionSvc)
	settingsH := rest.NewUserSettingsHandler(settingsSvc)

	r := router.New(authH, playerH, factionH, settingsH)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	factionStream, err := pubsubadapter.NewGCPMessageStream(ctx, cfg.PubsubProjectID, cfg.FactionPurchasedSubscription)
	if err != nil {
		return fmt.Errorf("faction-purchased stream: %w", err)
	}
	defer func() {
		if cerr := factionStream.Close(); cerr != nil {
			slog.Warn("faction-purchased stream close", "error", cerr)
		}
	}()

	premiumStream, err := pubsubadapter.NewGCPMessageStream(ctx, cfg.PubsubProjectID, cfg.PremiumUpdatedSubscription)
	if err != nil {
		return fmt.Errorf("premium-updated stream: %w", err)
	}
	defer func() {
		if cerr := premiumStream.Close(); cerr != nil {
			slog.Warn("premium-updated stream close", "error", cerr)
		}
	}()

	onboardedStream, err := pubsubadapter.NewGCPMessageStream(ctx, cfg.PubsubProjectID, cfg.PlayerOnboardedSubscription)
	if err != nil {
		return fmt.Errorf("player-onboarded stream: %w", err)
	}
	defer func() {
		if cerr := onboardedStream.Close(); cerr != nil {
			slog.Warn("player-onboarded stream close", "error", cerr)
		}
	}()

	factionSub := pubsubadapter.NewFactionPurchasedSubscriber(factionStream, factionRepo, txManager, eventRepo)
	premiumSub := pubsubadapter.NewPremiumUpdatedSubscriber(premiumStream, playerRepo, txManager, eventRepo)
	onboardedSub := pubsubadapter.NewPlayerOnboardedSubscriber(onboardedStream, factionSvc)

	slog.Info("account starting",
		"addr", srv.Addr,
		"pubsub_project", cfg.PubsubProjectID,
		"firestore_project", cfg.FirestoreProjectID,
	)

	return runHTTPAndSubscribers(ctx, srv, factionSub, premiumSub, onboardedSub)
}

// runHTTPAndSubscribers は HTTP server と Pub/Sub subscriber 群を並行起動し、
// どれかの失敗・シグナル到来で全体を graceful に停止する。
func runHTTPAndSubscribers(ctx context.Context, srv *http.Server, factionSub, premiumSub, onboardedSub subscriber) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := factionSub.Start(gCtx); err != nil && gCtx.Err() == nil {
			return fmt.Errorf("faction-purchased subscriber: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := premiumSub.Start(gCtx); err != nil && gCtx.Err() == nil {
			return fmt.Errorf("premium-updated subscriber: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := onboardedSub.Start(gCtx); err != nil && gCtx.Err() == nil {
			return fmt.Errorf("player-onboarded subscriber: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("shutdown requested")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	})

	return g.Wait()
}

// subscriber は Pub/Sub subscriber の起動インターフェース。
type subscriber interface {
	Start(ctx context.Context) error
}
