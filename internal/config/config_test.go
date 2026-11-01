package config_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/config"
)

const rsaKeyBits = 2048

func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	require.NoError(t, os.Unsetenv(name))
}

type envFixture struct {
	publicKey    *rsa.PublicKey
	publicKeyPEM string
}

func newEnvFixture(t *testing.T) *envFixture {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	return &envFixture{
		publicKey:    &privateKey.PublicKey,
		publicKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})),
	}
}

func (f *envFixture) validEnv() map[string]string {
	return map[string]string{
		"PORT":                      "8080",
		"DATABASE_CONN":             "postgres://tst-user:tst-pass@tst-host:5432/tst-db",
		"GOOGLE_CLOUD_PROJECT_ID":   "tst-project",
		"INTERNAL_AUTH_PUBLIC_KEY":  f.publicKeyPEM,
		"LOG_MODE":                  "local",
		"DATABASE_IAM_AUTH_ENABLED": "false",
	}
}

func (f *envFixture) loadConfig(t *testing.T, overrides map[string]string, unsetNames ...string) (*config.Config, error) {
	t.Helper()
	for name, value := range f.validEnv() {
		t.Setenv(name, value)
	}
	unsetEnv(t, "CLOUDSQL_CONNECTION_NAME")
	for name, value := range overrides {
		t.Setenv(name, value)
	}
	for _, name := range unsetNames {
		unsetEnv(t, name)
	}
	return config.New()
}

func (f *envFixture) mustLoadConfig(t *testing.T, overrides map[string]string, unsetNames ...string) *config.Config {
	t.Helper()
	cfg, err := f.loadConfig(t, overrides, unsetNames...)
	require.NoError(t, err)
	return cfg
}

type envCase struct {
	name       string
	overrides  map[string]string
	unsetNames []string
}

type invalidEnvCase struct {
	name        string
	overrides   map[string]string
	unsetNames  []string
	wantMessage string
}

func (f *envFixture) runInvalidEnvCases(t *testing.T, cases []invalidEnvCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.loadConfig(t, tt.overrides, tt.unsetNames...)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantMessage)
		})
	}
}

func TestNew(t *testing.T) {
	f := newEnvFixture(t)

	t.Run("[起動設定]環境変数の読み込み", func(t *testing.T) {
		t.Run("他の環境変数がすべて有効なとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				t.Run("PORTが1のとき、ポート番号は1になる", func(t *testing.T) {
					cfg := f.mustLoadConfig(t, map[string]string{"PORT": "1"})
					assert.Equal(t, 1, cfg.Port)
				})

				t.Run("PORTが65535のとき、ポート番号は65535になる", func(t *testing.T) {
					cfg := f.mustLoadConfig(t, map[string]string{"PORT": "65535"})
					assert.Equal(t, 65535, cfg.Port)
				})

				t.Run("DATABASE_CONNがtst-database-conn-valueのとき、データベース接続文字列はtst-database-conn-valueになる", func(t *testing.T) {
					cfg := f.mustLoadConfig(t, map[string]string{"DATABASE_CONN": "tst-database-conn-value"})
					assert.Equal(t, "tst-database-conn-value", cfg.DatabaseConn)
				})

				t.Run("GOOGLE_CLOUD_PROJECT_IDがtst-project-id-valueのとき、GoogleCloudプロジェクトIDはtst-project-id-valueになる", func(t *testing.T) {
					cfg := f.mustLoadConfig(t, map[string]string{"GOOGLE_CLOUD_PROJECT_ID": "tst-project-id-value"})
					assert.Equal(t, "tst-project-id-value", cfg.GoogleCloudProjectID)
				})

				t.Run("INTERNAL_AUTH_PUBLIC_KEYにRSA公開鍵のPEMを設定したとき、内部認証の公開鍵は設定したRSA公開鍵になる", func(t *testing.T) {
					cfg := f.mustLoadConfig(t, map[string]string{"INTERNAL_AUTH_PUBLIC_KEY": f.publicKeyPEM})
					assert.True(t, f.publicKey.Equal(cfg.InternalAuthPublicKey))
				})

				t.Run("LOG_MODEがproductionのとき、ログ出力形式はproductionになる", func(t *testing.T) {
					cfg := f.mustLoadConfig(t, map[string]string{"LOG_MODE": "production"})
					assert.Equal(t, config.LogMode("production"), cfg.LogMode)
				})

				t.Run("LOG_MODEがlocalのとき、ログ出力形式はlocalになる", func(t *testing.T) {
					cfg := f.mustLoadConfig(t, map[string]string{"LOG_MODE": "local"})
					assert.Equal(t, config.LogMode("local"), cfg.LogMode)
				})

				t.Run("DATABASE_IAM_AUTH_ENABLEDがtrueのとき", func(t *testing.T) {
					iamEnv := map[string]string{
						"DATABASE_IAM_AUTH_ENABLED": "true",
						"CLOUDSQL_CONNECTION_NAME":  "tst-project:tst-region:tst-instance",
					}

					t.Run("CLOUDSQL_CONNECTION_NAMEがtst-project:tst-region:tst-instanceのとき、IAM認証は有効になる", func(t *testing.T) {
						cfg := f.mustLoadConfig(t, iamEnv)
						assert.True(t, cfg.IsDatabaseIAMAuthEnabled)
					})

					t.Run("CLOUDSQL_CONNECTION_NAMEがtst-project:tst-region:tst-instanceのとき、CloudSQLの接続名はtst-project:tst-region:tst-instanceになる", func(t *testing.T) {
						cfg := f.mustLoadConfig(t, iamEnv)
						assert.Equal(t, "tst-project:tst-region:tst-instance", cfg.CloudSQLConnectionName)
					})
				})

				t.Run("DATABASE_IAM_AUTH_ENABLEDがfalseのとき", func(t *testing.T) {
					cases := []envCase{
						{
							name:       "CLOUDSQL_CONNECTION_NAMEが未設定のとき、IAM認証は無効になる",
							overrides:  map[string]string{"DATABASE_IAM_AUTH_ENABLED": "false"},
							unsetNames: []string{"CLOUDSQL_CONNECTION_NAME"},
						},
						{
							name: "CLOUDSQL_CONNECTION_NAMEがtst-project:tst-region:tst-instanceのとき、IAM認証は無効になる",
							overrides: map[string]string{
								"DATABASE_IAM_AUTH_ENABLED": "false",
								"CLOUDSQL_CONNECTION_NAME":  "tst-project:tst-region:tst-instance",
							},
						},
					}
					for _, tt := range cases {
						t.Run(tt.name, func(t *testing.T) {
							cfg := f.mustLoadConfig(t, tt.overrides, tt.unsetNames...)
							assert.False(t, cfg.IsDatabaseIAMAuthEnabled)
						})
					}
				})
			})

			t.Run("異常系", func(t *testing.T) {
				t.Run("PORTが無いとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "未設定のとき、PORTが必須であることを示すエラーが返る",
							unsetNames:  []string{"PORT"},
							wantMessage: "PORT is required",
						},
						{
							name:        "空文字のとき、PORTが必須であることを示すエラーが返る",
							overrides:   map[string]string{"PORT": ""},
							wantMessage: "PORT is required",
						},
					})
				})

				t.Run("PORTが整数でないとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "abcのとき、PORTが整数として読めないことを示すエラーが返る",
							overrides:   map[string]string{"PORT": "abc"},
							wantMessage: `PORT "abc"`,
						},
						{
							name:        "80.5のとき、PORTが整数として読めないことを示すエラーが返る",
							overrides:   map[string]string{"PORT": "80.5"},
							wantMessage: `PORT "80.5"`,
						},
					})
				})

				t.Run("PORTが範囲外の整数のとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "0のとき、PORTが1から65535の範囲外であることを示すエラーが返る",
							overrides:   map[string]string{"PORT": "0"},
							wantMessage: "PORT must be in 1-65535",
						},
						{
							name:        "65536のとき、PORTが1から65535の範囲外であることを示すエラーが返る",
							overrides:   map[string]string{"PORT": "65536"},
							wantMessage: "PORT must be in 1-65535",
						},
					})
				})

				t.Run("DATABASE_CONNが無いとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "未設定のとき、DATABASE_CONNが必須であることを示すエラーが返る",
							unsetNames:  []string{"DATABASE_CONN"},
							wantMessage: "DATABASE_CONN is required",
						},
						{
							name:        "空文字のとき、DATABASE_CONNが必須であることを示すエラーが返る",
							overrides:   map[string]string{"DATABASE_CONN": ""},
							wantMessage: "DATABASE_CONN is required",
						},
					})
				})

				t.Run("GOOGLE_CLOUD_PROJECT_IDが無いとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "未設定のとき、GOOGLE_CLOUD_PROJECT_IDが必須であることを示すエラーが返る",
							unsetNames:  []string{"GOOGLE_CLOUD_PROJECT_ID"},
							wantMessage: "GOOGLE_CLOUD_PROJECT_ID is required",
						},
						{
							name:        "空文字のとき、GOOGLE_CLOUD_PROJECT_IDが必須であることを示すエラーが返る",
							overrides:   map[string]string{"GOOGLE_CLOUD_PROJECT_ID": ""},
							wantMessage: "GOOGLE_CLOUD_PROJECT_ID is required",
						},
					})
				})

				t.Run("INTERNAL_AUTH_PUBLIC_KEYが無いとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "未設定のとき、INTERNAL_AUTH_PUBLIC_KEYが必須であることを示すエラーが返る",
							unsetNames:  []string{"INTERNAL_AUTH_PUBLIC_KEY"},
							wantMessage: "INTERNAL_AUTH_PUBLIC_KEY is required",
						},
						{
							name:        "空文字のとき、INTERNAL_AUTH_PUBLIC_KEYが必須であることを示すエラーが返る",
							overrides:   map[string]string{"INTERNAL_AUTH_PUBLIC_KEY": ""},
							wantMessage: "INTERNAL_AUTH_PUBLIC_KEY is required",
						},
					})
				})

				t.Run("INTERNAL_AUTH_PUBLIC_KEYがtst-not-pemのとき、INTERNAL_AUTH_PUBLIC_KEYが不正であることを示すエラーが返る", func(t *testing.T) {
					_, err := f.loadConfig(t, map[string]string{"INTERNAL_AUTH_PUBLIC_KEY": "tst-not-pem"})
					require.Error(t, err)
					assert.ErrorContains(t, err, "INTERNAL_AUTH_PUBLIC_KEY is invalid")
				})

				t.Run("LOG_MODEが不正なとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "未設定のとき、LOG_MODEはproductionまたはlocalでなければならないことを示すエラーが返る",
							unsetNames:  []string{"LOG_MODE"},
							wantMessage: `LOG_MODE must be "production" or "local"`,
						},
						{
							name:        "空文字のとき、LOG_MODEはproductionまたはlocalでなければならないことを示すエラーが返る",
							overrides:   map[string]string{"LOG_MODE": ""},
							wantMessage: `LOG_MODE must be "production" or "local"`,
						},
						{
							name:        "stagingのとき、LOG_MODEはproductionまたはlocalでなければならないことを示すエラーが返る",
							overrides:   map[string]string{"LOG_MODE": "staging"},
							wantMessage: `LOG_MODE must be "production" or "local"`,
						},
						{
							name:        "Productionのとき、LOG_MODEはproductionまたはlocalでなければならないことを示すエラーが返る",
							overrides:   map[string]string{"LOG_MODE": "Production"},
							wantMessage: `LOG_MODE must be "production" or "local"`,
						},
					})
				})

				t.Run("DATABASE_IAM_AUTH_ENABLEDが不正なとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "未設定のとき、DATABASE_IAM_AUTH_ENABLEDはtrueまたはfalseでなければならないことを示すエラーが返る",
							unsetNames:  []string{"DATABASE_IAM_AUTH_ENABLED"},
							wantMessage: `DATABASE_IAM_AUTH_ENABLED must be "true" or "false"`,
						},
						{
							name:        "空文字のとき、DATABASE_IAM_AUTH_ENABLEDはtrueまたはfalseでなければならないことを示すエラーが返る",
							overrides:   map[string]string{"DATABASE_IAM_AUTH_ENABLED": ""},
							wantMessage: `DATABASE_IAM_AUTH_ENABLED must be "true" or "false"`,
						},
						{
							name:        "Trueのとき、DATABASE_IAM_AUTH_ENABLEDはtrueまたはfalseでなければならないことを示すエラーが返る",
							overrides:   map[string]string{"DATABASE_IAM_AUTH_ENABLED": "True"},
							wantMessage: `DATABASE_IAM_AUTH_ENABLED must be "true" or "false"`,
						},
						{
							name:        "1のとき、DATABASE_IAM_AUTH_ENABLEDはtrueまたはfalseでなければならないことを示すエラーが返る",
							overrides:   map[string]string{"DATABASE_IAM_AUTH_ENABLED": "1"},
							wantMessage: `DATABASE_IAM_AUTH_ENABLED must be "true" or "false"`,
						},
					})
				})

				t.Run("DATABASE_IAM_AUTH_ENABLEDがtrueでCLOUDSQL_CONNECTION_NAMEが無いとき", func(t *testing.T) {
					f.runInvalidEnvCases(t, []invalidEnvCase{
						{
							name:        "未設定のとき、IAM認証の有効時はCLOUDSQL_CONNECTION_NAMEが必須であることを示すエラーが返る",
							overrides:   map[string]string{"DATABASE_IAM_AUTH_ENABLED": "true"},
							unsetNames:  []string{"CLOUDSQL_CONNECTION_NAME"},
							wantMessage: "CLOUDSQL_CONNECTION_NAME is required when DATABASE_IAM_AUTH_ENABLED is true",
						},
						{
							name:        "空文字のとき、IAM認証の有効時はCLOUDSQL_CONNECTION_NAMEが必須であることを示すエラーが返る",
							overrides:   map[string]string{"DATABASE_IAM_AUTH_ENABLED": "true", "CLOUDSQL_CONNECTION_NAME": ""},
							wantMessage: "CLOUDSQL_CONNECTION_NAME is required when DATABASE_IAM_AUTH_ENABLED is true",
						},
					})
				})
			})
		})
	})
}
