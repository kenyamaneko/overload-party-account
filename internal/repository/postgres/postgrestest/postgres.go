// Package postgrestest は DB を用いる account サービスのテストヘルパを提供する。
//
// postgres:16-alpine の Testcontainers を起動して実 PostgreSQL に対してテストを
// 実行する。コンテナはパッケージ単位で 1 回だけ起動し（RunMain）、ケースごとに
// スキーマ適用済みのテンプレートから専用のデータベースを複製して（NewDatabase）、
// 他のケースのデータと混ざらない状態から始める。
package postgrestest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	templateDatabaseName  = "schema_template"
	caseDatabasePrefix    = "case_"
	caseDatabaseSuffixLen = 8
	startupTimeout        = 60 * time.Second
)

// Postgres は起動済み PostgreSQL コンテナと、ケース専用データベースを作る管理用の接続プールのハンドル。
type Postgres struct {
	Container *postgres.PostgresContainer
	adminPool *pgxpool.Pool
	adminDSN  string
}

// Database はケース専用のデータベースへの接続情報。
type Database struct {
	Pool             *pgxpool.Pool
	ConnectionString string
}

type config struct {
	schemaFiles []string
}

// Option は Start の振る舞いを構成する。
type Option func(*config)

// WithSchemaFile は repo-root 起点の相対パスでスキーマ SQL を登録する。
// 順序通りに Exec で適用される。複数指定可（cross-service schema 参照を
// 再現したい場合に使う）。
func WithSchemaFile(repoRelativePath string) Option {
	return func(c *config) {
		c.schemaFiles = append(c.schemaFiles, repoRelativePath)
	}
}

// Start は postgres:16-alpine コンテナを起動し、登録された schema SQL を適用した
// テンプレートデータベースを用意して返す。終了時は Close を呼ぶ（RunMain が代行）。
func Start(ctx context.Context, opts ...Option) (*Postgres, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	root, err := findRepoRoot()
	if err != nil {
		return nil, err
	}

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(startupTimeout),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	adminDSN, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("container connection string: %w", err), container.Terminate(ctx))
	}

	adminPool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("pgxpool new: %w", err), container.Terminate(ctx))
	}

	pg := &Postgres{Container: container, adminPool: adminPool, adminDSN: adminDSN}
	if err := pg.createTemplate(ctx, root, cfg.schemaFiles); err != nil {
		adminPool.Close()
		return nil, errors.Join(err, container.Terminate(ctx))
	}
	return pg, nil
}

// createTemplate はスキーマ SQL を適用したテンプレートデータベースを作る。
// 複製の元に接続が残っていると複製できないため、適用が済んだら接続を閉じる。
func (p *Postgres) createTemplate(ctx context.Context, root string, schemaFiles []string) error {
	if _, err := p.adminPool.Exec(ctx, "CREATE DATABASE "+templateDatabaseName); err != nil {
		return fmt.Errorf("create template database: %w", err)
	}
	templatePool, err := newPoolForDatabase(ctx, p.adminDSN, templateDatabaseName)
	if err != nil {
		return err
	}
	defer templatePool.Close()

	for _, rel := range schemaFiles {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read schema %s: %w", path, err)
		}
		if _, err := templatePool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("apply schema %s: %w", path, err)
		}
	}
	return nil
}

// Close は接続プールをクローズしコンテナを終了する。
func (p *Postgres) Close(ctx context.Context) error {
	p.adminPool.Close()
	if err := p.Container.Terminate(ctx); err != nil {
		return fmt.Errorf("terminate container: %w", err)
	}
	return nil
}

// NewDatabase はスキーマ適用済みのテンプレートからケース専用のデータベースを複製して返す。
// ケースの終了時に、そのデータベースだけを削除する。
func (p *Postgres) NewDatabase(t *testing.T) *Database {
	t.Helper()
	ctx := context.Background()

	suffix := make([]byte, caseDatabaseSuffixLen/2)
	_, err := rand.Read(suffix)
	require.NoError(t, err, "generate database name")
	name := caseDatabasePrefix + hex.EncodeToString(suffix)

	_, err = p.adminPool.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateDatabaseName)
	require.NoError(t, err, "create case database")
	dsn, err := dsnForDatabase(p.adminDSN, name)
	require.NoError(t, err, "case database dsn")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "connect case database")
	t.Cleanup(func() {
		pool.Close()
		_, dropErr := p.adminPool.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		assert.NoError(t, dropErr, "drop case database")
	})
	return &Database{Pool: pool, ConnectionString: dsn}
}

// RunMain は TestMain のボイラープレートを集約する。コンテナ起動 → m.Run →
// クリーンアップを保証する。各テストパッケージは
// `os.Exit(postgrestest.RunMain(m, &sharedPg, opts...))` と書き、ケースごとに
// `sharedPg.NewDatabase(t)` で専用のデータベースを得る。
func RunMain(m *testing.M, out **Postgres, opts ...Option) int {
	ctx := context.Background()

	pg, err := Start(ctx, opts...)
	if err != nil {
		slog.Error("postgrestest start", "error", err)
		return 1
	}
	*out = pg

	defer func() {
		if cerr := pg.Close(ctx); cerr != nil {
			slog.Warn("postgrestest close", "error", cerr)
		}
	}()

	return m.Run()
}

func newPoolForDatabase(ctx context.Context, baseDSN, database string) (*pgxpool.Pool, error) {
	dsn, err := dsnForDatabase(baseDSN, database)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect database %s: %w", database, err)
	}
	return pool, nil
}

func dsnForDatabase(baseDSN, database string) (string, error) {
	parsed, err := url.Parse(baseDSN)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// findRepoRoot は本ファイルの位置（postgrestest パッケージは
// internal/repository/postgres/postgrestest 配下に置かれる規約）から go.mod を
// 持つディレクトリを探索して返す。build tag やソースキャッシュに左右されないよう
// runtime.Caller を anchor とする。
func findRepoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found from postgrestest")
		}
		dir = parent
	}
}
