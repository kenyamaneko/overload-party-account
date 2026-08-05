//go:build integration

package usecase

import (
	"context"
	"testing"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

func TestApplyNameSet(t *testing.T) {
	t.Run("onboarding-name-setの適用", func(t *testing.T) {
		t.Run("未処理のイベントを適用すると、表示名が保存されオンボード状態がname_setに進み処理済みになる", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "", false) // name 未確定 (NULL) のプレイヤー

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "10000000-1111-1111-1111-111111111111"
			processed, err := svc.ApplyNameSet(ctx, eventID, apiscenario.EventTypeOnboardingNameSet, testPlayerID1, "Kenya")
			require.NoError(t, err)
			assert.True(t, processed)

			p, ferr := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, p.Name)
			assert.Equal(t, "Kenya", *p.Name)

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusNameSet, status)
		})

		t.Run("同一event_idを再配信すると、処理済みにならず表示名は変わらない", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "", false)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "20000000-1111-1111-1111-111111111111"
			_, err := svc.ApplyNameSet(ctx, eventID, apiscenario.EventTypeOnboardingNameSet, testPlayerID1, "Kenya")
			require.NoError(t, err)

			processed, err := svc.ApplyNameSet(ctx, eventID, apiscenario.EventTypeOnboardingNameSet, testPlayerID1, "Renamed")
			require.NoError(t, err)
			assert.False(t, processed)

			p, ferr := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, p.Name)
			assert.Equal(t, "Kenya", *p.Name, "重複配信は副作用を起こさず最初の値のまま")
		})

		t.Run("オンボード完了済みのプレイヤーに遅延到着すると、表示名は更新されるが状態はcompletedから後退しない", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Kenya", false)
			updateOnboardingStatus(t, testPlayerID1, domain.OnboardingStatusCompleted)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "30000000-1111-1111-1111-111111111111"
			processed, err := svc.ApplyNameSet(ctx, eventID, apiscenario.EventTypeOnboardingNameSet, testPlayerID1, "Renamed")
			require.NoError(t, err)
			assert.True(t, processed)

			p, ferr := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, p.Name)
			assert.Equal(t, "Renamed", *p.Name)

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusCompleted, status)
		})

		t.Run("表示名が空白のみのとき、エラーになり何も保存されない", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "", false)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "40000000-1111-1111-1111-111111111111"
			processed, err := svc.ApplyNameSet(ctx, eventID, apiscenario.EventTypeOnboardingNameSet, testPlayerID1, "   ")
			require.ErrorIs(t, err, domain.ErrInvalidName)
			assert.False(t, processed)

			p, ferr := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, ferr)
			assert.Nil(t, p.Name)

			assert.False(t, isProcessedEvent(t, eventID))

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusNotStarted, status)
		})

		t.Run("存在しないプレイヤーのとき、エラーになり処理済みにならない", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "50000000-1111-1111-1111-111111111111"
			processed, err := svc.ApplyNameSet(ctx, eventID, apiscenario.EventTypeOnboardingNameSet, testPlayerID2, "Kenya")
			require.ErrorIs(t, err, port.ErrNotFound)
			assert.False(t, processed)

			assert.False(t, isProcessedEvent(t, eventID), "player 未存在での失敗は Tx ロールバックで processed_events も巻き戻る")
		})
	})
}

func TestApplyFactionSet(t *testing.T) {
	t.Run("onboarding-faction-setの適用", func(t *testing.T) {
		t.Run("name未確定 (NULL)でもinitial_factionが反映されprocessedになる", func(t *testing.T) {
			// name はシナリオが入力時点で確定済みのため、faction-set 経路は initial_faction の
			// 反映と冪等ガードのみを担い、name には触れない。
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "", false) // name 未確定 (NULL) のプレイヤー

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			processed, err := svc.ApplyFactionSet(
				ctx,
				"44444444-4444-4444-4444-444444444444",
				"onboarding.faction-set",
				testPlayerID1,
				"SHE",
			)
			require.NoError(t, err)
			assert.True(t, processed)

			// faction だけ反映され、name は触られず NULL のまま。
			p, ferr := playerRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, ferr)
			assert.Nil(t, p.Name, "ApplyFactionSet は name を書かない")

			initial, ferr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, initial)
			assert.Equal(t, "SHE", *initial)

			factions, ferr := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, ferr)
			assert.ElementsMatch(t, []string{"SHE"}, factions)
		})

		t.Run("name_setまで進んだプレイヤーに適用すると、状態がfaction_setに進む", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			updateOnboardingStatus(t, testPlayerID1, domain.OnboardingStatusNameSet)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			processed, err := svc.ApplyFactionSet(
				ctx,
				"55555555-5555-5555-5555-555555555555",
				"onboarding.faction-set",
				testPlayerID1,
				"SHE",
			)
			require.NoError(t, err)
			assert.True(t, processed)

			initial, ferr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, initial)
			assert.Equal(t, "SHE", *initial)

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusFactionSet, status)
		})

		t.Run("同一event_idを再配信すると、処理済みにならず初期陣営と状態は変わらない", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			updateOnboardingStatus(t, testPlayerID1, domain.OnboardingStatusNameSet)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "66666666-6666-6666-6666-666666666666"
			_, err := svc.ApplyFactionSet(ctx, eventID, "onboarding.faction-set", testPlayerID1, "SHE")
			require.NoError(t, err)

			processed, err := svc.ApplyFactionSet(ctx, eventID, "onboarding.faction-set", testPlayerID1, "SHE")
			require.NoError(t, err)
			assert.False(t, processed)

			initial, ferr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, initial)
			assert.Equal(t, "SHE", *initial)

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusFactionSet, status)
		})

		t.Run("既にSHEで確定済みのプレイヤーに別のTenkiが届くと、ErrFactionConflictになりSHEのまま残る", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			updateOnboardingStatus(t, testPlayerID1, domain.OnboardingStatusNameSet)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			firstEventID := "77777777-7777-7777-7777-777777777777"
			_, err := svc.ApplyFactionSet(ctx, firstEventID, "onboarding.faction-set", testPlayerID1, "SHE")
			require.NoError(t, err)

			secondEventID := "88888888-8888-8888-8888-888888888888"
			_, err = svc.ApplyFactionSet(ctx, secondEventID, "onboarding.faction-set", testPlayerID1, "Tenki")
			require.ErrorIs(t, err, ErrFactionConflict)

			initial, ferr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, initial)
			assert.Equal(t, "SHE", *initial)

			assert.False(t, isProcessedEvent(t, secondEventID), "衝突時は Tx ロールバックで processed_events も巻き戻る")
		})

		t.Run("既にSHEで確定済みのプレイヤーに同じSHEが別event_idで届くと、上書きなしで処理済みになる", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			updateOnboardingStatus(t, testPlayerID1, domain.OnboardingStatusNameSet)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			firstEventID := "99999999-9999-9999-9999-999999999999"
			_, err := svc.ApplyFactionSet(ctx, firstEventID, "onboarding.faction-set", testPlayerID1, "SHE")
			require.NoError(t, err)

			secondEventID := "aaaaaaaa-1111-1111-1111-111111111111"
			processed, err := svc.ApplyFactionSet(ctx, secondEventID, "onboarding.faction-set", testPlayerID1, "SHE")
			require.NoError(t, err)
			assert.True(t, processed)

			initial, ferr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, ferr)
			require.NotNil(t, initial)
			assert.Equal(t, "SHE", *initial)
		})

		t.Run("選択不可のNeutralのとき、エラーになり初期陣営は未設定のまま保存されない", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "bbbbbbbb-1111-1111-1111-111111111111"
			processed, err := svc.ApplyFactionSet(ctx, eventID, "onboarding.faction-set", testPlayerID1, "Neutral")
			require.ErrorIs(t, err, ErrInvalidFaction)
			assert.False(t, processed)

			initial, ferr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, ferr)
			assert.Nil(t, initial)

			assert.False(t, isProcessedEvent(t, eventID))

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusNotStarted, status)
		})
	})
}

func TestApplyCompleted(t *testing.T) {
	t.Run("player-onboardedの適用", func(t *testing.T) {
		t.Run("未処理の完了イベントを適用すると、オンボード状態がcompletedになり処理済みになる", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			processed, err := svc.ApplyCompleted(ctx, "cccccccc-1111-1111-1111-111111111111", "player_onboarded", testPlayerID1)
			require.NoError(t, err)
			assert.True(t, processed)

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusCompleted, status)
		})

		t.Run("同一event_idを再適用すると、処理済みにならず状態はcompletedのまま", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "dddddddd-1111-1111-1111-111111111111"
			_, err := svc.ApplyCompleted(ctx, eventID, "player_onboarded", testPlayerID1)
			require.NoError(t, err)

			processed, err := svc.ApplyCompleted(ctx, eventID, "player_onboarded", testPlayerID1)
			require.NoError(t, err)
			assert.False(t, processed)

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusCompleted, status)
		})

		t.Run("既にcompletedのプレイヤーに別event_idで再到着すると、処理済みになり状態はcompletedのまま", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			updateOnboardingStatus(t, testPlayerID1, domain.OnboardingStatusCompleted)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			processed, err := svc.ApplyCompleted(ctx, "eeeeeeee-1111-1111-1111-111111111111", "player_onboarded", testPlayerID1)
			require.NoError(t, err)
			assert.True(t, processed)

			status, serr := playerRepo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, serr)
			assert.Equal(t, domain.OnboardingStatusCompleted, status)
		})

		t.Run("存在しないプレイヤーのとき、エラーになり処理済み記録も残らない", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)

			playerRepo, _, factionRepo, _, tx := newRealRepos()
			eventRepo := newProcessedEventRepo()
			svc := NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)

			eventID := "ffffffff-1111-1111-1111-111111111111"
			processed, err := svc.ApplyCompleted(ctx, eventID, "player_onboarded", testPlayerID2)
			require.ErrorIs(t, err, port.ErrNotFound)
			assert.False(t, processed)

			assert.False(t, isProcessedEvent(t, eventID))
		})
	})
}

// updateOnboardingStatus はシード済みプレイヤーの onboarding_status を直接書き換える。
// seedPlayer が既定で not_started を挿入するため、前進済み状態を前提とするケースで使う。
func updateOnboardingStatus(t *testing.T, playerID, status string) {
	t.Helper()
	_, err := sharedPg.Pool.Exec(context.Background(),
		`UPDATE account.players SET onboarding_status = $1 WHERE player_id = $2`,
		status, playerID)
	require.NoError(t, err)
}

// isProcessedEvent は event_id が account.processed_events に commit 済みで存在するかを返す。
func isProcessedEvent(t *testing.T, eventID string) bool {
	t.Helper()
	var count int
	require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM account.processed_events WHERE event_id = $1`, eventID,
	).Scan(&count))
	return count > 0
}
