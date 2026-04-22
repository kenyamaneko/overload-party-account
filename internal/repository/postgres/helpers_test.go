//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// seedPlayer は account.players + player_daily_battle + player_progression の最小シードを投入する。
// テストで頻繁に必要な「ログイン済みプレイヤー」の状態を 1 行で作る。
// Level/Exp は Player アグリゲートに詰めて返すが、物理的には player_progression に INSERT される。
func seedPlayer(t *testing.T, playerID, firebaseUID, username string, isPremium bool) *apiaccount.Player {
	t.Helper()
	now := time.Now().UTC()
	p := &apiaccount.Player{
		PlayerID:    playerID,
		FirebaseUID: firebaseUID,
		Username:    username,
		Level:       1,
		Exp:         0,
		IsPremium:   isPremium,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	ctx := context.Background()
	_, err := sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, username, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		p.PlayerID, p.FirebaseUID, p.Username, p.IsPremium,
		p.EquippedIconNo, p.SelectedFaction, p.PremiumExpiresAt, p.CreatedAt, p.UpdatedAt)
	require.NoError(t, err)

	_, err = sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.player_progression (player_id, level, exp)
		 VALUES ($1, $2, $3)`,
		p.PlayerID, p.Level, p.Exp)
	require.NoError(t, err)

	today := civil.DateOf(now)
	_, err = sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.player_daily_battle (player_id, daily_battle_count, last_reset_date)
		 VALUES ($1, $2, $3)`,
		p.PlayerID, 0, time.Date(today.Year, today.Month, today.Day, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return p
}

// seedUserSettings は account.user_settings に 1 行追加する。
func seedUserSettings(t *testing.T, playerID, language string, bgm, se int64, push bool) {
	t.Helper()
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.user_settings (player_id, language, bgm_volume, se_volume, push_enabled)
		 VALUES ($1, $2, $3, $4, $5)`,
		playerID, language, bgm, se, push)
	require.NoError(t, err)
}
