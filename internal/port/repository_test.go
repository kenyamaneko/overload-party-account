package port_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

func ptr[T any](v T) *T {
	return &v
}

func TestPlayerSettingsPatchSpecified(t *testing.T) {
	t.Run("[プレイヤー設定]更新項目の指定判定", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("更新項目が指定されているか判定すると", func(t *testing.T) {
				tests := []struct {
					name      string
					patch     port.PlayerSettingsPatch
					wantEmpty bool
				}{
					{name: "言語・BGM音量・SE音量・通知許可のすべてが未指定のとき、指定なしという結果になる", patch: port.PlayerSettingsPatch{}, wantEmpty: true},
					{name: "言語だけを「en」で指定したとき、指定ありという結果になる", patch: port.PlayerSettingsPatch{Language: ptr("en")}},
					{name: "言語だけを空文字で指定したとき、指定ありという結果になる", patch: port.PlayerSettingsPatch{Language: ptr("")}},
					{name: "BGM音量だけを0で指定したとき、指定ありという結果になる", patch: port.PlayerSettingsPatch{BgmVolume: ptr(int64(0))}},
					{name: "SE音量だけを0で指定したとき、指定ありという結果になる", patch: port.PlayerSettingsPatch{SeVolume: ptr(int64(0))}},
					{name: "通知許可だけを無効で指定したとき、指定ありという結果になる", patch: port.PlayerSettingsPatch{IsPushEnabled: ptr(false)}},
					{
						name: "言語・BGM音量・SE音量・通知許可のすべてを指定したとき、指定ありという結果になる",
						patch: port.PlayerSettingsPatch{
							Language: ptr("en"), BgmVolume: ptr(int64(80)), SeVolume: ptr(int64(20)), IsPushEnabled: ptr(true),
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						assert.Equal(t, tt.wantEmpty, tt.patch.IsEmpty())
					})
				}
			})
		})
	})
}
