// Package firestore は Firestore を使った、ゲーム設定の読み取りの実装を提供する。
package firestore

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

const gameConfigCollection = "game_config"

var _ port.GameConfigRepo = (*GameConfigRepository)(nil)

// GameConfigRepository はゲーム設定の Firestore 実装である。
type GameConfigRepository struct {
	client *firestore.Client
}

// New は GameConfigRepository を生成する。
func New(client *firestore.Client) *GameConfigRepository {
	return &GameConfigRepository{client: client}
}

// GetInt64 はキーに対応するゲーム設定の整数値を返す。
// 設定がなければ port.ErrNotFound、値の項目がない・null・整数でないときは port.ErrValueNotInteger を返す。
// 呼び出しが取り消されたときは context.Canceled、期限を過ぎたときは context.DeadlineExceeded を包んだエラーを返す。
func (r *GameConfigRepository) GetInt64(ctx context.Context, key string) (int64, error) {
	snap, err := r.client.Collection(gameConfigCollection).Doc(key).Get(ctx)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return 0, fmt.Errorf("game config %q: %w", key, port.ErrNotFound)
		case codes.Canceled:
			return 0, fmt.Errorf("get game config %q: %w: %w", key, context.Canceled, err)
		case codes.DeadlineExceeded:
			return 0, fmt.Errorf("get game config %q: %w: %w", key, context.DeadlineExceeded, err)
		default:
			return 0, fmt.Errorf("get game config %q: %w", key, err)
		}
	}

	var doc struct {
		Value *int64 `firestore:"value"`
	}
	if err := snap.DataTo(&doc); err != nil {
		return 0, fmt.Errorf("game config %q: %w: %w", key, port.ErrValueNotInteger, err)
	}
	if doc.Value == nil {
		return 0, fmt.Errorf("game config %q has no value: %w", key, port.ErrValueNotInteger)
	}
	return *doc.Value, nil
}
