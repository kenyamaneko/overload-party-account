//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

var errWriteFailed = errors.New("書き込み失敗")

func writeInTx(ctx context.Context, r *postgrestest.Repositories, failWith error) error {
	player := postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
		player.Name = postgrestest.NewPointer("テスト太郎")
	})
	createErr := r.Players.Create(ctx, player, postgrestest.NewProgression(postgrestest.PlayerID1))
	settingsErr := r.Settings.InsertPlayerSettings(ctx, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, "ja", 50, 50, true))
	_, eventErr := r.Events.InsertProcessedEvent(ctx, postgrestest.EventID1, "test_event_a")
	return errors.Join(createErr, settingsErr, eventErr, failWith)
}

func runWriteInTx(ctx context.Context, r *postgrestest.Repositories, failWith error) error {
	return r.TxManager.RunInTx(ctx, func(ctx context.Context) error {
		return writeInTx(ctx, r, failWith)
	})
}

func runUnregisteredSettingsWriteInTx(ctx context.Context, r *postgrestest.Repositories) error {
	return r.TxManager.RunInTx(ctx, func(ctx context.Context) error {
		createErr := r.Players.Create(ctx,
			postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
			postgrestest.NewProgression(postgrestest.PlayerID1))
		settingsErr := r.Settings.InsertPlayerSettings(ctx, postgrestest.NewSettings(postgrestest.UnregisteredPlayerID))
		return errors.Join(createErr, settingsErr)
	})
}

func newCancelledContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

func TestTransactionRun(t *testing.T) {
	t.Run("[トランザクション]書き込みの確定と巻き戻し", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("トランザクション内の書き込みがエラーなしで終わったとき", func(t *testing.T) {
				t.Run("トランザクション内の書き込みで表示名「テスト太郎」のプレイヤーを登録していると、完了後にプレイヤーIDで取得した結果の表示名が「テスト太郎」になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					require.NoError(t, runWriteInTx(t.Context(), r, nil))

					got, err := r.Players.GetByID(t.Context(), postgrestest.PlayerID1)

					require.NoError(t, err)
					assert.Equal(t, postgrestest.NewPointer("テスト太郎"), got.Name)
				})
				t.Run("トランザクション内の書き込みで言語「ja」のプレイヤー設定を登録していると、完了後にプレイヤーIDで取得した結果の言語が「ja」になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					require.NoError(t, runWriteInTx(t.Context(), r, nil))

					got, err := r.Settings.GetPlayerSettings(t.Context(), postgrestest.PlayerID1)

					require.NoError(t, err)
					assert.Equal(t, "ja", got.Language)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("トランザクション内の書き込みがエラー「書き込み失敗」を返したとき", func(t *testing.T) {
				t.Run("エラー「書き込み失敗」が原因のエラーが返る", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)

					err := runWriteInTx(t.Context(), r, errWriteFailed)

					assert.ErrorIs(t, err, errWriteFailed)
				})
				t.Run("トランザクション内の書き込みで行ったプレイヤー設定登録は残らず、プレイヤーIDで取得すると見つからないエラーになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					require.ErrorIs(t, runWriteInTx(t.Context(), r, errWriteFailed), errWriteFailed)

					_, err := r.Settings.GetPlayerSettings(t.Context(), postgrestest.PlayerID1)

					assert.ErrorIs(t, err, port.ErrNotFound)
				})
				t.Run("トランザクション内の書き込みで記録した処理済みイベントIDは残らず、同じイベントIDをもう一度記録すると新規に記録されたという結果が返る", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					require.ErrorIs(t, runWriteInTx(t.Context(), r, errWriteFailed), errWriteFailed)

					got, err := r.Events.InsertProcessedEvent(t.Context(), postgrestest.EventID1, "test_event_a")

					require.NoError(t, err)
					assert.True(t, got)
				})
				t.Run("エラー「書き込み失敗」を返した後に、表示名「テスト太郎」のプレイヤー登録と言語「ja」のプレイヤー設定登録と処理済みイベントIDの記録を行うトランザクション内の書き込みを、エラーを返さずに実行すると", func(t *testing.T) {
					t.Run("完了後にプレイヤーIDで取得した結果の表示名が「テスト太郎」になる", func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						require.ErrorIs(t, runWriteInTx(t.Context(), r, errWriteFailed), errWriteFailed)
						require.NoError(t, runWriteInTx(t.Context(), r, nil))

						got, err := r.Players.GetByID(t.Context(), postgrestest.PlayerID1)

						require.NoError(t, err)
						assert.Equal(t, postgrestest.NewPointer("テスト太郎"), got.Name)
					})
					t.Run("完了後にプレイヤーIDで取得した結果の言語が「ja」になる", func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						require.ErrorIs(t, runWriteInTx(t.Context(), r, errWriteFailed), errWriteFailed)
						require.NoError(t, runWriteInTx(t.Context(), r, nil))

						got, err := r.Settings.GetPlayerSettings(t.Context(), postgrestest.PlayerID1)

						require.NoError(t, err)
						assert.Equal(t, "ja", got.Language)
					})
					t.Run("完了後に、トランザクション内の書き込みで記録した処理済みイベントIDをもう一度記録すると、既に記録済みという結果が返る", func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						require.ErrorIs(t, runWriteInTx(t.Context(), r, errWriteFailed), errWriteFailed)
						require.NoError(t, runWriteInTx(t.Context(), r, nil))

						got, err := r.Events.InsertProcessedEvent(t.Context(), postgrestest.EventID1, "test_event_a")

						require.NoError(t, err)
						assert.False(t, got)
					})
				})
			})
			t.Run("プレイヤーの登録を含む書き込みが失敗した後に、プレイヤーIDで取得すると", func(t *testing.T) {
				tests := []struct {
					name string
					run  func(t *testing.T, r *postgrestest.Repositories) error
				}{
					{
						name: "トランザクション内の書き込みがエラー「書き込み失敗」を返したとき、トランザクション内の書き込みで行ったプレイヤー登録は残らず、見つからないエラーになる",
						run: func(t *testing.T, r *postgrestest.Repositories) error {
							return runWriteInTx(t.Context(), r, errWriteFailed)
						},
					},
					{
						name: "トランザクション内の書き込みでプレイヤーを登録した後に、未登録のプレイヤーIDのプレイヤー設定を登録して失敗したとき、トランザクション内の書き込みで行ったプレイヤー登録は残らず、見つからないエラーになる",
						run: func(t *testing.T, r *postgrestest.Repositories) error {
							return runUnregisteredSettingsWriteInTx(t.Context(), r)
						},
					},
					{
						name: "呼び出しが取り消し済みのとき、トランザクション内の書き込みで行ったプレイヤー登録は残らず、見つからないエラーになる",
						run: func(t *testing.T, r *postgrestest.Repositories) error {
							return runWriteInTx(newCancelledContext(t), r, nil)
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						require.Error(t, tt.run(t, r))

						_, err := r.Players.GetByID(t.Context(), postgrestest.PlayerID1)

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
			t.Run("トランザクション内の書き込みでプレイヤーを登録した後に、未登録のプレイヤーIDのプレイヤー設定を登録して失敗したとき、存在しないプレイヤーを指すエラーが返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				err := runUnregisteredSettingsWriteInTx(t.Context(), r)

				postgrestest.RequirePgError(t, err, postgrestest.SQLStateForeignKeyViolation)
			})
			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーが返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				err := runWriteInTx(newCancelledContext(t), r, nil)

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}
