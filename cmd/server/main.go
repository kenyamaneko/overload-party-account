package main

import (
	"context"
	"errors"
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

	"github.com/kenyamaneko/overload-party-account/internal/adapter/internalauth"
	pubsubadapter "github.com/kenyamaneko/overload-party-account/internal/adapter/pubsub"
	"github.com/kenyamaneko/overload-party-account/internal/config"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	accountfirestore "github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("account fatal", "error", err)
		os.Exit(1)
	}
}

// setupLogger は LOG_MODE に応じてグローバル slog ロガーを初期化する。
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
// slog のデフォルトフィールド名 (level/msg) は Cloud Logging が認識しないので severity/message に変換する。
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

	pool, err := pgxpool.New(ctx, cfg.DatabaseConn)
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
	playerViewRepo := postgres.NewPlayerViewRepository(pool)
	playerSettingsRepo := postgres.NewPlayerSettingsRepository(pool)
	gameConfigRepo := accountfirestore.NewGameConfigRepository(fsClient)
	if err := usecase.ValidateGameConfig(ctx, gameConfigRepo); err != nil {
		return fmt.Errorf("validate game_config: %w", err)
	}
	factionRepo := postgres.NewFactionRepository(pool)
	eventRepo := postgres.NewProcessedEventRepository(pool)

	// playerRepo (*postgres.PlayerRepository) は責務別 interface
	// (PlayerRepo / PlayerPremiumRepo / PlayerOnboardingRepo / PlayerProgressionRepo / PlayerBattleRepo)
	// すべてを暗黙的に満たすため、同じインスタンスを複数引数に渡せる (Go の structural typing)。
	authInteractor := usecase.NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, gameConfigRepo, txManager)
	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, playerViewRepo, gameConfigRepo, txManager)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, txManager)
	onboardingInteractor := usecase.NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, txManager)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(playerSettingsRepo)

	authH := rest.NewAuthHandler(authInteractor)
	playerH := rest.NewPlayerHandler(playerInteractor)
	factionH := rest.NewFactionHandler(factionInteractor)
	settingsH := rest.NewPlayerSettingsHandler(settingsInteractor)

	authVerifier := internalauth.NewVerifier(
		internalauth.StaticHS256Resolver([]byte(cfg.InternalAuthSecret), internalauth.DefaultKeyID),
	)

	r := router.New(authH, playerH, factionH, settingsH, authVerifier)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	streams, closeStreams, err := openSubscriptionStreams(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStreams()

	factionSub := pubsubadapter.NewFactionAcquiredSubscriber(streams.FactionAcquired, factionRepo, txManager, eventRepo)
	premiumSub := pubsubadapter.NewPremiumUpdatedSubscriber(streams.PremiumUpdated, playerRepo, txManager, eventRepo)
	onboardedSub := pubsubadapter.NewPlayerOnboardedSubscriber(streams.PlayerOnboarded, onboardingInteractor)
	nameSetSub := pubsubadapter.NewOnboardingNameSetSubscriber(streams.OnboardingNameSet, onboardingInteractor)
	factionSetSub := pubsubadapter.NewOnboardingFactionSetSubscriber(streams.OnboardingFactionSet, onboardingInteractor)

	slog.Info("account starting",
		"addr", srv.Addr,
		"pubsub_project", cfg.PubsubProjectID,
		"firestore_project", cfg.FirestoreProjectID,
	)

	return runHTTPAndSubscribers(ctx, srv, factionSub, premiumSub, onboardedSub, nameSetSub, factionSetSub)
}

// runHTTPAndSubscribers は HTTP server と Pub/Sub subscriber 群を並行起動し、
// どれかの失敗・シグナル到来で全体を graceful に停止する。
func runHTTPAndSubscribers(ctx context.Context, srv *http.Server, subscribers ...port.EventSubscriber) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	for _, sub := range subscribers {
		g.Go(func() error {
			if err := sub.Start(gCtx); err != nil && gCtx.Err() == nil {
				return fmt.Errorf("subscriber: %w", err)
			}
			return nil
		})
	}

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

// subscriptionStreams は account サービスが購読する 5 つの Pub/Sub stream を束ねる。
// stream ごとの defer Close を main から消し、一括 close 関数を返すための受け皿。
type subscriptionStreams struct {
	FactionAcquired      *pubsubadapter.Stream
	PremiumUpdated       *pubsubadapter.Stream
	PlayerOnboarded      *pubsubadapter.Stream
	OnboardingNameSet    *pubsubadapter.Stream
	OnboardingFactionSet *pubsubadapter.Stream
}

// openSubscriptionStreams は 5 つの Pub/Sub stream を順次 Open し、一括 close 関数と
// 共に返す。途中で失敗した場合はそれまでに開いた stream を逆順に Close してから
// エラーを返すため、リーク無しに run() の早期 return が可能。
func openSubscriptionStreams(ctx context.Context, cfg *config.Config) (*subscriptionStreams, func(), error) {
	specs := []struct {
		name           string
		subscriptionID string
		assign         func(*subscriptionStreams, *pubsubadapter.Stream)
	}{
		{"faction-acquired", cfg.FactionAcquiredSubscription, func(s *subscriptionStreams, st *pubsubadapter.Stream) { s.FactionAcquired = st }},
		{"premium-updated", cfg.PremiumUpdatedSubscription, func(s *subscriptionStreams, st *pubsubadapter.Stream) { s.PremiumUpdated = st }},
		{"player-onboarded", cfg.PlayerOnboardedSubscription, func(s *subscriptionStreams, st *pubsubadapter.Stream) { s.PlayerOnboarded = st }},
		{"onboarding-name-set", cfg.OnboardingNameSetSubscription, func(s *subscriptionStreams, st *pubsubadapter.Stream) { s.OnboardingNameSet = st }},
		{"onboarding-faction-set", cfg.OnboardingFactionSetSubscription, func(s *subscriptionStreams, st *pubsubadapter.Stream) { s.OnboardingFactionSet = st }},
	}

	streams := &subscriptionStreams{}
	opened := make([]struct {
		name   string
		stream *pubsubadapter.Stream
	}, 0, len(specs))

	closeAll := func() {
		// 逆順 Close: 後から開いた SDK リソースを先に解放する慣用。
		for i := len(opened) - 1; i >= 0; i-- {
			if cerr := opened[i].stream.Close(); cerr != nil {
				slog.Warn("pubsub stream close", "name", opened[i].name, "error", cerr)
			}
		}
	}

	for _, spec := range specs {
		st, err := pubsubadapter.NewStream(ctx, cfg.PubsubProjectID, spec.subscriptionID)
		if err != nil {
			closeAll()
			return nil, nil, fmt.Errorf("%s stream: %w", spec.name, err)
		}
		spec.assign(streams, st)
		opened = append(opened, struct {
			name   string
			stream *pubsubadapter.Stream
		}{name: spec.name, stream: st})
	}

	return streams, closeAll, nil
}
