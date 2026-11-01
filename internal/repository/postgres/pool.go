package postgres

import (
	"context"
	"errors"
	"fmt"
	"net"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPasswordAuthPool は、接続文字列の資格情報で接続するプールと、プールを閉じる関数を返す。
func NewPasswordAuthPool(ctx context.Context, databaseConn string) (*pgxpool.Pool, func() error, error) {
	pool, err := pgxpool.New(ctx, databaseConn)
	if err != nil {
		return nil, nil, fmt.Errorf("pgxpool new: %w", err)
	}
	return pool, func() error {
		pool.Close()
		return nil
	}, nil
}

// NewIAMAuthPool は、Cloud SQL への IAM 認証で接続するプールと、プールと接続の仲介を閉じる関数を返す。
func NewIAMAuthPool(ctx context.Context, databaseConn string, cloudSQLConnectionName string) (*pgxpool.Pool, func() error, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseConn)
	if err != nil {
		return nil, nil, fmt.Errorf("parse database conn: %w", err)
	}

	dialer, err := cloudsqlconn.NewDialer(ctx,
		cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("cloudsqlconn new dialer: %w", err)
	}

	poolCfg.ConnConfig.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.Dial(ctx, cloudSQLConnectionName)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("pgxpool new with config: %w", err), closeCloudSQLDialer(dialer))
	}

	return pool, func() error {
		pool.Close()
		return closeCloudSQLDialer(dialer)
	}, nil
}

func closeCloudSQLDialer(dialer *cloudsqlconn.Dialer) error {
	if err := dialer.Close(); err != nil {
		return fmt.Errorf("cloudsqlconn dialer close: %w", err)
	}
	return nil
}
