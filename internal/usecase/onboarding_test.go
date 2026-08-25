//go:build integration

package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"
	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

func TestOnboardingInteractor_ApplyNameSet(t *testing.T) {
	t.Run("OnboardingInteractor", func(t *testing.T) {
		t.Run("ApplyNameSetに共通する仕様", func(t *testing.T) {
			t.Run("同一event_idが未処理のとき、処理を実行し、戻り値のprocessedはtrueになり、オンボーディング状態は目標状態(name_set)へ進む", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-1")

				processed, err := interactor.ApplyNameSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingNameSet, playerID, "プレイヤー")

				require.NoError(t, err)
				assert.True(t, processed)
				status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, domain.OnboardingStatusNameSet, status)
			})

			t.Run("同一event_idが処理済みのとき、追加の副作用を起こさず、戻り値のprocessedはfalseになる", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-2")
				eventID := uuid.NewString()
				_, err := interactor.ApplyNameSet(context.Background(), eventID, apiscenario.EventTypeOnboardingNameSet, playerID, "最初の名前")
				require.NoError(t, err)

				processed, err := interactor.ApplyNameSet(context.Background(), eventID, apiscenario.EventTypeOnboardingNameSet, playerID, "別の名前")

				require.NoError(t, err)
				assert.False(t, processed)
				player, err := postgres.NewPlayerRepository(sharedPg.Pool).FindByID(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, player.Name)
				assert.Equal(t, "最初の名前", *player.Name)
			})

			t.Run("処理本体の途中でエラーが起きたとき、event_idの処理済み記録もロールバックされ、同一event_idで再配信されると再度処理が実行される", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				eventID := uuid.NewString()
				nonexistentPlayerID := uuid.NewString()
				_, err := interactor.ApplyNameSet(context.Background(), eventID, apiscenario.EventTypeOnboardingNameSet, nonexistentPlayerID, "プレイヤー")
				require.Error(t, err)

				playerID := registerTestPlayer(t, "firebase-onb-3")
				processed, err := interactor.ApplyNameSet(context.Background(), eventID, apiscenario.EventTypeOnboardingNameSet, playerID, "プレイヤー")

				require.NoError(t, err)
				assert.True(t, processed)
			})

			t.Run("目標状態への遷移が後退にあたるとき、オンボーディング状態を変更しない", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-4")
				require.NoError(t, postgres.NewPlayerRepository(sharedPg.Pool).UpdateOnboardingStatus(context.Background(), playerID, domain.OnboardingStatusCompleted))

				processed, err := interactor.ApplyNameSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingNameSet, playerID, "プレイヤー")

				require.NoError(t, err)
				assert.True(t, processed)
				status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, domain.OnboardingStatusCompleted, status)
			})
		})

		t.Run("ApplyNameSet固有の仕様", func(t *testing.T) {
			t.Run("表示名が表示名バリデーションの規定に違反するとき、処理を実行せずエラーを返す", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-5")

				_, err := interactor.ApplyNameSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingNameSet, playerID, "")

				require.Error(t, err)
			})

			t.Run("表示名が有効なとき、対象プレイヤーの表示名を指定値に更新する", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-6")

				_, err := interactor.ApplyNameSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingNameSet, playerID, "新しい名前")

				require.NoError(t, err)
				player, err := postgres.NewPlayerRepository(sharedPg.Pool).FindByID(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, player.Name)
				assert.Equal(t, "新しい名前", *player.Name)
			})
		})
	})
}

func TestOnboardingInteractor_ApplyFactionSet(t *testing.T) {
	t.Run("OnboardingInteractor", func(t *testing.T) {
		t.Run("ApplyFactionSet固有の仕様", func(t *testing.T) {
			t.Run("初期選択ファクションIDが選択可能なファクションでないとき、処理を実行せずエラーを返す", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-faction-1")

				_, err := interactor.ApplyFactionSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingFactionSet, playerID, gamedesign.FactionNeutral)

				require.Error(t, err)
			})

			t.Run("対象プレイヤーが初期ファクション未選択のとき、指定したファクションを初期ファクションとして設定する", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-faction-2")

				processed, err := interactor.ApplyFactionSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingFactionSet, playerID, gamedesign.FactionSHE)

				require.NoError(t, err)
				assert.True(t, processed)
				got, err := postgres.NewFactionRepository(sharedPg.Pool).GetInitialFaction(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, gamedesign.FactionSHE, *got)
			})

			t.Run("対象プレイヤーが既に別のファクションを初期ファクションとして選択済みのとき、ErrFactionConflictを返し、初期ファクションは変更しない", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-faction-3")
				require.NoError(t, postgres.NewFactionRepository(sharedPg.Pool).SetInitialFaction(context.Background(), playerID, gamedesign.FactionSHE))

				_, err := interactor.ApplyFactionSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingFactionSet, playerID, gamedesign.FactionTenki)

				assert.ErrorIs(t, err, usecase.ErrFactionConflict)
				got, err := postgres.NewFactionRepository(sharedPg.Pool).GetInitialFaction(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, gamedesign.FactionSHE, *got)
			})

			t.Run("対象プレイヤーが既に同じファクションを初期ファクションとして選択済みのとき(再配信)、ErrFactionConflictを返さない", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-faction-4")
				require.NoError(t, postgres.NewFactionRepository(sharedPg.Pool).SetInitialFaction(context.Background(), playerID, gamedesign.FactionSHE))

				_, err := interactor.ApplyFactionSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingFactionSet, playerID, gamedesign.FactionSHE)

				assert.NotErrorIs(t, err, usecase.ErrFactionConflict)
			})

			t.Run("対象プレイヤーが既に同じファクションを初期ファクションとして選択済みのとき(再配信)、オンボーディング状態の前進は行われる", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-faction-5")
				require.NoError(t, postgres.NewFactionRepository(sharedPg.Pool).SetInitialFaction(context.Background(), playerID, gamedesign.FactionSHE))

				_, err := interactor.ApplyFactionSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingFactionSet, playerID, gamedesign.FactionSHE)

				require.NoError(t, err)
				status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, domain.OnboardingStatusFactionSet, status)
			})
		})
	})
}

func TestOnboardingInteractor_ApplyCompleted(t *testing.T) {
	t.Run("OnboardingInteractor", func(t *testing.T) {
		t.Run("同一event_idが未処理のとき、処理を実行し、戻り値のprocessedはtrueになり、オンボーディング状態は目標状態(completed)へ進む", func(t *testing.T) {
			interactor := newTestOnboardingInteractor(t)
			playerID := registerTestPlayer(t, "firebase-onb-completed-1")

			processed, err := interactor.ApplyCompleted(context.Background(), uuid.NewString(), apiscenario.EventTypePlayerOnboarded, playerID)

			require.NoError(t, err)
			assert.True(t, processed)
			status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
			require.NoError(t, err)
			assert.Equal(t, domain.OnboardingStatusCompleted, status)
		})
	})
}
