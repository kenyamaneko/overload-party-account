package usecase

import (
	"context"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// ensurePlayerExists はプレイヤーが登録されていることを確かめる。登録されていなければ port.ErrNotFound を包んだエラーを返す。
func ensurePlayerExists(ctx context.Context, playerRepo port.PlayerRepo, playerID string) error {
	isFound, err := playerRepo.Exists(ctx, playerID)
	if err != nil {
		return fmt.Errorf("check player exists: %w", err)
	}
	if !isFound {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}
