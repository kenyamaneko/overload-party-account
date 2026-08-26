package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// fakeGameConfigRepo は port.GameConfigRepo を満たすメモリ内 fake。
type fakeGameConfigRepo struct {
	values map[string]int64
}

func newFakeGameConfigRepo(values map[string]int64) *fakeGameConfigRepo {
	return &fakeGameConfigRepo{values: values}
}

func (r *fakeGameConfigRepo) GetInt64(ctx context.Context, key string) (int64, error) {
	v, ok := r.values[key]
	if !ok {
		return 0, port.ErrNotFound
	}
	return v, nil
}

func validGameConfigValues() map[string]int64 {
	return map[string]int64{
		"free_daily_battle_limit": 5,
		"exp_formula_coefficient": 100,
		"exp_win":                 50,
		"exp_loss":                10,
		"exp_draw":                20,
	}
}

func TestValidateGameConfig(t *testing.T) {
	t.Run("起動時ゲーム設定検証", func(t *testing.T) {
		t.Run("free_daily_battle_limitの値が0以下のとき、エラーを返す", func(t *testing.T) {
			values := validGameConfigValues()
			values["free_daily_battle_limit"] = 0
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.Error(t, err)
		})

		t.Run("exp_formula_coefficientの値が0以下のとき、エラーを返す", func(t *testing.T) {
			values := validGameConfigValues()
			values["exp_formula_coefficient"] = 0
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.Error(t, err)
		})

		t.Run("free_daily_battle_limitのキーが存在しないとき、エラーを返す", func(t *testing.T) {
			values := validGameConfigValues()
			delete(values, "free_daily_battle_limit")
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.Error(t, err)
		})

		t.Run("exp_formula_coefficientのキーが存在しないとき、エラーを返す", func(t *testing.T) {
			values := validGameConfigValues()
			delete(values, "exp_formula_coefficient")
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.Error(t, err)
		})

		t.Run("exp_win/exp_loss/exp_drawの値が0であっても、エラーにならない", func(t *testing.T) {
			values := validGameConfigValues()
			values["exp_win"] = 0
			values["exp_loss"] = 0
			values["exp_draw"] = 0
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.NoError(t, err)
		})

		t.Run("exp_winのキーが存在しないとき、エラーを返す", func(t *testing.T) {
			values := validGameConfigValues()
			delete(values, "exp_win")
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.Error(t, err)
		})

		t.Run("exp_lossのキーが存在しないとき、エラーを返す", func(t *testing.T) {
			values := validGameConfigValues()
			delete(values, "exp_loss")
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.Error(t, err)
		})

		t.Run("exp_drawのキーが存在しないとき、エラーを返す", func(t *testing.T) {
			values := validGameConfigValues()
			delete(values, "exp_draw")
			repo := newFakeGameConfigRepo(values)

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.Error(t, err)
		})

		t.Run("free_daily_battle_limitとexp_formula_coefficientが0より大きく、exp_win/exp_loss/exp_drawのキーが全て存在するとき、エラーを返さない", func(t *testing.T) {
			repo := newFakeGameConfigRepo(validGameConfigValues())

			err := usecase.ValidateGameConfig(context.Background(), repo)

			assert.NoError(t, err)
		})
	})
}
