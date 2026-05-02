//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

// seededPlayer はテスト helper の戻り値専用の集約ビュー (Player + Level/Exp)。
// account.players と account.player_progression の両テーブルを 1 シードで作るため、
// 検証側で「name を確認したい」「level/exp を確認したい」が 1 つのオブジェクトで済む。
type seededPlayer struct {
	domain.Player
	Level int64
	Exp   int64
}

// seedPlayer は account.players + player_progression の最小シードを投入する。
// テストで頻繁に必要な「ログイン済みプレイヤー」の状態を 1 行で作る。
// player_daily_battle はゲーム日単位の履歴台帳になったため seedPlayer では作らない
// (バトル発生時に IncrementDailyBattleCount で UPSERT される)。
// 引数 name に "" を渡すと name を NULL として挿入する (オンボーディング前の未確定状態を再現する用途)。
func seedPlayer(t *testing.T, playerID, firebaseUID, name string, isPremium bool) *seededPlayer {
	t.Helper()
	now := time.Now().UTC()
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	p := &seededPlayer{
		Player: domain.Player{
			PlayerID:    playerID,
			FirebaseUID: firebaseUID,
			Name:        namePtr,
			IsPremium:   isPremium,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Level: 1,
		Exp:   0,
	}
	ctx := context.Background()
	_, err := sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, name, is_premium, equipped_icon_no, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.PlayerID, p.FirebaseUID, p.Name, p.IsPremium,
		p.EquippedIconNo, p.PremiumExpiresAt, p.CreatedAt, p.UpdatedAt)
	require.NoError(t, err)

	_, err = sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.player_progression (player_id, level, exp)
		 VALUES ($1, $2, $3)`,
		p.PlayerID, p.Level, p.Exp)
	require.NoError(t, err)

	return p
}

// seedPlayerDailyBattle は account.player_daily_battle に 1 行追加する。
// usecase の上限判定のように「すでに当日カウントが N」の状態を直接作るために使う。
func seedPlayerDailyBattle(t *testing.T, playerID string, gameDate civil.Date, count int64) {
	t.Helper()
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.player_daily_battle (player_id, game_date, daily_battle_count)
		 VALUES ($1, $2, $3)`,
		playerID,
		time.Date(gameDate.Year, gameDate.Month, gameDate.Day, 0, 0, 0, 0, time.UTC),
		count,
	)
	require.NoError(t, err)
}

// seedPlayerFaction は account.player_factions に 1 行追加する。
func seedPlayerFaction(t *testing.T, playerID, faction string, isInitial bool) {
	t.Helper()
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.player_factions (player_id, faction, is_initial) VALUES ($1, $2, $3)`,
		playerID, faction, isInitial)
	require.NoError(t, err)
}

// seedPlayerSettings は account.player_settings に 1 行追加する。
func seedPlayerSettings(t *testing.T, playerID, language string, bgm, se int64, push bool) {
	t.Helper()
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.player_settings (player_id, language, bgm_volume, se_volume, push_enabled)
		 VALUES ($1, $2, $3, $4, $5)`,
		playerID, language, bgm, se, push)
	require.NoError(t, err)
}

// playerNamePtr は nil を許容して Player.Name (*string) を取り出すテスト用アクセサ。
// 「該当行なし → 名前なし」を *string の nil で表現するため、テスト本体に if を持ち込まずに
// 有/無パターンを assert.Equal で比較できるようにする。
func playerNamePtr(p *domain.Player) *string {
	if p == nil {
		return nil
	}
	return p.Name
}

// playerIsPremium は nil を許容して Player.IsPremium を取り出すテスト用アクセサ。
// 該当行なしは false で表現する (UpdatePremium の有/無パターン用)。
func playerIsPremium(p *domain.Player) bool {
	if p == nil {
		return false
	}
	return p.IsPremium
}

// progressionLevelExp は nil を許容して PlayerProgression から Level/Exp を取り出す。
// 該当行なしは (0, 0)。
func progressionLevelExp(p *domain.PlayerProgression) (level, exp int64) {
	if p == nil {
		return 0, 0
	}
	return p.Level, p.Exp
}

// viewLevelExp は nil を許容して PlayerView から Level/Exp を取り出す。
// 該当行なしは (0, 0)。FindByID 経由 (JOIN) の永続化検証で使う。
func viewLevelExp(v *domain.PlayerView) (level, exp int64) {
	if v == nil {
		return 0, 0
	}
	return v.Level, v.Exp
}

// settingsSnapshot は PlayerSettings の比較対象フィールド (UpdatedAt を除く) を
// 抜き出すスナップショット型。time.Time の µs 精度差で assert.Equal が誤判定するのを
// 避けつつ、nil/non-nil 両方を 1 つの assert.Equal で比較できるようにする。
type settingsSnapshot struct {
	Language    string
	BgmVolume   int64
	SeVolume    int64
	PushEnabled bool
}

// snapshotSettings は nil を許容して PlayerSettings をスナップショット化する。
// 該当行なしは nil で表現し、テスト本体に if を持ち込まずに有/無パターンを
// assert.Equal で比較できるようにする。
func snapshotSettings(s *domain.PlayerSettings) *settingsSnapshot {
	if s == nil {
		return nil
	}
	return &settingsSnapshot{
		Language:    s.Language,
		BgmVolume:   s.BgmVolume,
		SeVolume:    s.SeVolume,
		PushEnabled: s.PushEnabled,
	}
}
