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
	t.Run("[オンボーディング]オンボーディング進行の副作用反映", func(t *testing.T) {
		t.Run("ApplyNameSetに共通する仕様", func(t *testing.T) {
			t.Run("同一のイベントIDを処理するのが初めてのとき、処理を実行し、戻り値はtrueになり、オンボーディング状態は目標状態(name_set)へ進む", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-1")

				processed, err := interactor.ApplyNameSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingNameSet, playerID, "プレイヤー")

				require.NoError(t, err)
				assert.True(t, processed)
				status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
				require.NoError(t, err)
				assert.Equal(t, domain.OnboardingStatusNameSet, status)
			})

			t.Run("同一のイベントIDが処理済みのとき、追加の副作用を起こさず、戻り値はfalseになる", func(t *testing.T) {
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

			t.Run("処理本体の途中でエラーが起きたとき、イベントIDの処理済み記録もロールバックされ、同一のイベントIDで再配信されると再度処理が実行される", func(t *testing.T) {
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
			t.Run("表示名が無効(空文字・空白のみ・20文字超・制御文字のいずれか)なとき、処理を実行せずエラーを返す", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-5")

				_, err := interactor.ApplyNameSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingNameSet, playerID, "")

				require.Error(t, err)
				player, err := postgres.NewPlayerRepository(sharedPg.Pool).FindByID(context.Background(), playerID)
				require.NoError(t, err)
				assert.Nil(t, player.Name)
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
	t.Run("[オンボーディング]オンボーディング進行の副作用反映", func(t *testing.T) {
		t.Run("ApplyFactionSet固有の仕様", func(t *testing.T) {
			t.Run("初期選択ファクションIDが選択可能な4ファクション(SHE/Tenki/Sugar/Tuners)に含まれないNeutralのとき、処理を実行せずエラーを返す", func(t *testing.T) {
				interactor := newTestOnboardingInteractor(t)
				playerID := registerTestPlayer(t, "firebase-onb-faction-1")

				_, err := interactor.ApplyFactionSet(context.Background(), uuid.NewString(), apiscenario.EventTypeOnboardingFactionSet, playerID, gamedesign.FactionNeutral)

				require.Error(t, err)
				got, err := postgres.NewFactionRepository(sharedPg.Pool).GetInitialFaction(context.Background(), playerID)
				require.NoError(t, err)
				assert.Nil(t, got)
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
	t.Run("[オンボーディング]オンボーディング進行の副作用反映", func(t *testing.T) {
		t.Run("同一のイベントIDを処理するのが初めてのとき、処理を実行し、戻り値はtrueになり、オンボーディング状態は目標状態(completed)へ進む", func(t *testing.T) {
			interactor := newTestOnboardingInteractor(t)
			playerID := registerTestPlayer(t, "firebase-onb-completed-1")

			processed, err := interactor.ApplyCompleted(context.Background(), uuid.NewString(), apiscenario.EventTypePlayerOnboarded, playerID)

			require.NoError(t, err)
			assert.True(t, processed)
			status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
			require.NoError(t, err)
			assert.Equal(t, domain.OnboardingStatusCompleted, status)
		})

		t.Run("同一のイベントIDが処理済みのとき、戻り値はfalseになる", func(t *testing.T) {
			interactor := newTestOnboardingInteractor(t)
			playerID := registerTestPlayer(t, "firebase-onb-completed-2")
			eventID := uuid.NewString()
			_, err := interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, playerID)
			require.NoError(t, err)

			processed, err := interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, playerID)

			require.NoError(t, err)
			assert.False(t, processed)
		})

		t.Run("同一のイベントIDが処理済みのとき、オンボーディング状態はfaction_setのままになる", func(t *testing.T) {
			interactor := newTestOnboardingInteractor(t)
			playerID := registerTestPlayer(t, "firebase-onb-completed-3")
			eventID := uuid.NewString()
			_, err := interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, playerID)
			require.NoError(t, err)
			require.NoError(t, postgres.NewPlayerRepository(sharedPg.Pool).UpdateOnboardingStatus(context.Background(), playerID, domain.OnboardingStatusFactionSet))

			_, err = interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, playerID)

			require.NoError(t, err)
			status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
			require.NoError(t, err)
			assert.Equal(t, domain.OnboardingStatusFactionSet, status)
		})

		t.Run("対象プレイヤーが存在せずイベント処理が一度失敗したとき、同一のイベントIDで再配信されると、戻り値はtrueになる", func(t *testing.T) {
			interactor := newTestOnboardingInteractor(t)
			eventID := uuid.NewString()
			nonexistentPlayerID := uuid.NewString()
			_, err := interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, nonexistentPlayerID)
			require.Error(t, err)

			playerID := registerTestPlayer(t, "firebase-onb-completed-4")
			processed, err := interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, playerID)

			require.NoError(t, err)
			assert.True(t, processed)
		})

		t.Run("対象プレイヤーが存在せずイベント処理が一度失敗したとき、同一のイベントIDで再配信されると、オンボーディング状態は目標状態(completed)へ進む", func(t *testing.T) {
			interactor := newTestOnboardingInteractor(t)
			eventID := uuid.NewString()
			nonexistentPlayerID := uuid.NewString()
			_, err := interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, nonexistentPlayerID)
			require.Error(t, err)

			playerID := registerTestPlayer(t, "firebase-onb-completed-5")
			_, err = interactor.ApplyCompleted(context.Background(), eventID, apiscenario.EventTypePlayerOnboarded, playerID)

			require.NoError(t, err)
			status, err := postgres.NewPlayerRepository(sharedPg.Pool).GetOnboardingStatus(context.Background(), playerID)
			require.NoError(t, err)
			assert.Equal(t, domain.OnboardingStatusCompleted, status)
		})
	})
}
