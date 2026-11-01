//go:build integration

package pubsubpush_test

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/adapter/system"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

const (
	nameSetPublisherBugMessage         = "onboarding-name-set subscriber: publisher bug"
	factionSetPublisherBugMessage      = "onboarding-faction-set subscriber: publisher bug"
	playerOnboardedPublisherBugMessage = "player-onboarded subscriber: publisher bug"
	nameSetApplyFailedMessage          = "onboarding-name-set subscriber: apply failed"
	factionSetApplyFailedMessage       = "onboarding-faction-set subscriber: apply failed"
	playerOnboardedApplyFailedMessage  = "player-onboarded subscriber: apply failed"
)

func nameSetEventBody(name string) func(t *testing.T, playerID string) string {
	return func(t *testing.T, playerID string) string {
		return pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, name)
	}
}

func factionSetEventBody(faction string) func(t *testing.T, playerID string) string {
	return func(t *testing.T, playerID string) string {
		return pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.NewEventID(), playerID, faction)
	}
}

func completedEventBody(t *testing.T, playerID string) string {
	return pubsubpushtest.BuildOnboardedBody(t, pubsubpushtest.NewEventID(), playerID)
}

func newClosedPoolCase(t *testing.T) (closedPoolStack, openStack *pubsubpushtest.AccountStack, logs *loggingtest.Logs, playerID string) {
	t.Helper()
	db := sharedPg.NewDatabase(t)
	idGenerator := system.NewUUIDPlayerIDGenerator()
	openStack = pubsubpushtest.NewAccountStack(t, db.Pool, idGenerator)
	logs = loggingtest.Capture(t)
	playerID = openStack.Register(t)

	pool, err := pgxpool.NewWithConfig(t.Context(), db.Pool.Config())
	require.NoError(t, err)
	closedPoolStack = pubsubpushtest.NewAccountStack(t, pool, idGenerator)
	pool.Close()
	return closedPoolStack, openStack, logs, playerID
}

func pushEventToPlayer(path string, setup pubsubpushtest.PlayerSetup, buildBody func(t *testing.T, playerID string) string) func(t *testing.T) pubsubpushtest.Outcome {
	return func(t *testing.T) pubsubpushtest.Outcome {
		t.Helper()
		return pubsubpushtest.PushEventForPlayer(t, sharedPg, path, setup, buildBody)
	}
}

func pushNameSetEventWithClosedPool(t *testing.T) pubsubpushtest.Outcome {
	t.Helper()
	closedPoolStack, _, logs, playerID := newClosedPoolCase(t)
	rec := closedPoolStack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, "テスト太郎"))
	return pubsubpushtest.Outcome{Stack: closedPoolStack, Rec: rec, Logs: logs, PlayerID: playerID}
}

func redeliverNameSetEventAfterDatabaseUnavailable(t *testing.T) pubsubpushtest.Outcome {
	t.Helper()
	closedPoolStack, openStack, _, playerID := newClosedPoolCase(t)
	nameSetEventID := pubsubpushtest.NewEventID()
	closedPoolStack.PushRejected(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, nameSetEventID, playerID, "テスト太郎"))

	rec := openStack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, nameSetEventID, playerID, "テスト太郎"))

	return pubsubpushtest.Outcome{Stack: openStack, Rec: rec, PlayerID: playerID}
}

func redeliverNameSetEventAfterInvalidName(t *testing.T) pubsubpushtest.Outcome {
	t.Helper()
	stack, _ := pubsubpushtest.NewCase(t, sharedPg)
	playerID := stack.Register(t)
	nameSetEventID := pubsubpushtest.NewEventID()
	stack.PushRejected(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, nameSetEventID, playerID, "   "))

	rec := stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, nameSetEventID, playerID, "テスト太郎"))

	return pubsubpushtest.Outcome{Stack: stack, Rec: rec, PlayerID: playerID}
}

type failureClassificationCase struct {
	name  string
	push  func(t *testing.T) pubsubpushtest.Outcome
	check pubsubpushtest.OutcomeCheck
}

func runFailureClassificationCases(t *testing.T, cases []failureClassificationCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.push(t))
		})
	}
}

func TestOnboardingProgressFailureClassification(t *testing.T) {
	t.Run("[オンボーディング進行イベント]失敗原因の分類", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("イベントを受けると", func(t *testing.T) {
				t.Run("該当プレイヤーが存在しないために反映が失敗するとき", func(t *testing.T) {
					runFailureClassificationCases(t, []failureClassificationCase{
						{
							"表示名設定イベントのとき、ERRORログに「発行側起因の不整合」が記録される",
							pushEventToPlayer(pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.SetupUnregistered, nameSetEventBody("テスト太郎")),
							pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(nameSetPublisherBugMessage)),
						},
						{
							"陣営設定イベントのとき、ERRORログに「発行側起因の不整合」が記録される",
							pushEventToPlayer(pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.SetupUnregistered, factionSetEventBody("SHE")),
							pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(factionSetPublisherBugMessage)),
						},
						{
							"完了イベントのとき、ERRORログに「発行側起因の不整合」が記録される",
							pushEventToPlayer(pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.SetupUnregistered, completedEventBody),
							pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(playerOnboardedPublisherBugMessage)),
						},
					})
				})
				t.Run("オンボーディング進行状態が段階を飛ばしているために反映が失敗するとき", func(t *testing.T) {
					runFailureClassificationCases(t, []failureClassificationCase{
						{
							"陣営設定イベントで、オンボーディング進行状態が「not_started」のとき、ERRORログに「反映失敗」が記録される",
							pushEventToPlayer(pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.SetupRegistered, factionSetEventBody("SHE")),
							pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(factionSetApplyFailedMessage)),
						},
						{
							"完了イベントで、オンボーディング進行状態が「name_set」のとき、ERRORログに「反映失敗」が記録される",
							pushEventToPlayer(pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.SetupNameSet, completedEventBody),
							pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(playerOnboardedApplyFailedMessage)),
						},
						{
							"陣営設定イベントで、オンボーディング進行状態が「not_started」のとき、ERRORログのエラーの内容は、オンボーディング進行状態が段階を飛ばしていることを示す",
							pushEventToPlayer(pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.SetupRegistered, factionSetEventBody("SHE")),
							pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(factionSetApplyFailedMessage, "onboarding status skipped a stage")),
						},
						{
							"完了イベントで、オンボーディング進行状態が「name_set」のとき、ERRORログのエラーの内容は、オンボーディング進行状態が段階を飛ばしていることを示す",
							pushEventToPlayer(pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.SetupNameSet, completedEventBody),
							pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(playerOnboardedApplyFailedMessage, "onboarding status skipped a stage")),
						},
					})
				})
				runFailureClassificationCases(t, []failureClassificationCase{
					{
						"陣営設定イベントの反映が、初期陣営が「SHE」と確定済みなのにイベントの陣営が「Tenki」であるために失敗するとき、ERRORログに「発行側起因の不整合」が記録される",
						pushEventToPlayer(pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.SetupFactionSet, factionSetEventBody("Tenki")),
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(factionSetPublisherBugMessage)),
					},
					{
						"表示名設定イベントの反映が、表示名が半角スペース3つであるために失敗するとき、ERRORログに「発行側起因の不整合」が記録される",
						pushEventToPlayer(pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.SetupRegistered, nameSetEventBody("   ")),
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(nameSetPublisherBugMessage)),
					},
					{
						"陣営設定イベントの反映が、陣営が選択できない「Neutral」であるために失敗するとき、ERRORログに「発行側起因の不整合」が記録される",
						pushEventToPlayer(pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.SetupNameSet, factionSetEventBody("Neutral")),
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(factionSetPublisherBugMessage)),
					},
					{
						"表示名設定イベントの反映が、データベースに接続できないために失敗するとき、ERRORログに「反映失敗」が記録される",
						pushNameSetEventWithClosedPool,
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(nameSetApplyFailedMessage)),
					},
					{
						"陣営設定イベントの反映が、初期陣営が「SHE」と確定済みなのにイベントの陣営が「Tenki」であるために失敗するとき、ERRORログのエラーの内容は、別の初期陣営が確定済みであることを示す",
						pushEventToPlayer(pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.SetupFactionSet, factionSetEventBody("Tenki")),
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(factionSetPublisherBugMessage, "initial faction conflict")),
					},
					{
						"表示名設定イベントの反映が、表示名が半角スペース3つであるために失敗するとき、ERRORログのエラーの内容は、表示名が不正であることを示す",
						pushEventToPlayer(pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.SetupRegistered, nameSetEventBody("   ")),
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(nameSetPublisherBugMessage, "invalid name")),
					},
					{
						"陣営設定イベントの反映が、陣営が選択できない「Neutral」であるために失敗するとき、ERRORログのエラーの内容は、選択できない陣営であることを示す",
						pushEventToPlayer(pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.SetupNameSet, factionSetEventBody("Neutral")),
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(factionSetPublisherBugMessage, "invalid faction")),
					},
					{
						"表示名設定イベントの反映が、データベースに接続できないために失敗するとき、ERRORログのエラーの内容は、データベースに接続できないことを示す",
						pushNameSetEventWithClosedPool,
						pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(nameSetApplyFailedMessage, "database unavailable")),
					},
					{
						"表示名設定イベントの反映が、データベースに接続できないために失敗するとき、処理失敗を示す500の応答になる",
						pushNameSetEventWithClosedPool,
						pubsubpushtest.CheckPushResult(pubsubpushtest.RespondsHandlerFailed()),
					},
				})
			})
		})
	})
}

func TestOnboardingProgressRecoveryByRedelivery(t *testing.T) {
	t.Run("[オンボーディング進行イベント]失敗後の回復", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("オンボーディング進行状態が「name_set」のプレイヤーへの完了イベントが、陣営設定を経ていないため500になった後、陣営「SHE」の陣営設定イベントを反映してから、同じイベントIDの完了イベントが再配信されたとき", func(t *testing.T) {
				t.Run("再配信された完了イベントの応答は200になる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithNameSet(t, "テスト太郎")
					completedEventID := pubsubpushtest.NewEventID()
					stack.PushRejected(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, completedEventID, playerID))
					stack.PushAccepted(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.NewEventID(), playerID, "SHE"))

					rec := stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, completedEventID, playerID))

					pubsubpushtest.AssertStatus(t, http.StatusOK, rec)
				})

				t.Run("取得したプレイヤー情報のオンボーディング進行状態は「completed」になる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithNameSet(t, "テスト太郎")
					completedEventID := pubsubpushtest.NewEventID()
					stack.PushRejected(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, completedEventID, playerID))
					stack.PushAccepted(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.NewEventID(), playerID, "SHE"))

					stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, completedEventID, playerID))

					assert.Equal(t, "completed", stack.FetchPlayerView(t, playerID).Player.OnboardingStatus)
				})
			})

			t.Run("表示名設定イベントが500になった後、同じイベントIDで表示名「テスト太郎」の表示名設定イベントを2回目に受けたとき", func(t *testing.T) {
				cases := []struct {
					name      string
					redeliver func(t *testing.T) pubsubpushtest.Outcome
					check     pubsubpushtest.OutcomeCheck
				}{
					{"データベースに接続できないために500になり、2回目はデータベースに接続できるとき、2回目の表示名設定イベントの応答は200になる", redeliverNameSetEventAfterDatabaseUnavailable, pubsubpushtest.RespondsOK()},
					{"表示名が半角スペース3つのために500になったとき、2回目の表示名設定イベントの応答は200になる", redeliverNameSetEventAfterInvalidName, pubsubpushtest.RespondsOK()},
					{"データベースに接続できないために500になり、2回目はデータベースに接続できるとき、取得したプレイヤー情報のオンボーディング進行状態は「name_set」になる", redeliverNameSetEventAfterDatabaseUnavailable, hasOnboardingStatus("name_set")},
					{"表示名が半角スペース3つのために500になったとき、取得したプレイヤー情報のオンボーディング進行状態は「name_set」になる", redeliverNameSetEventAfterInvalidName, hasOnboardingStatus("name_set")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						tc.check(t, tc.redeliver(t))
					})
				}
			})
		})
	})
}
