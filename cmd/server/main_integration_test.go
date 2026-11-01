//go:build integration

package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
	"github.com/kenyamaneko/overload-party-account/internal/repository/firestore/firestoretest"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

const (
	processExitTimeout   = 30 * time.Second
	healthReadyTimeout   = 30 * time.Second
	logAppearTimeout     = 10 * time.Second
	pollInterval         = 50 * time.Millisecond
	httpClientTimeout    = 5 * time.Second
	rsaKeyBits           = 2048
	tokenValidDuration   = time.Hour
	internalAuthHeader   = "X-Internal-Auth"
	internalAuthKeyID    = "v1"
	internalAuthIssuer   = "overload-party-gateway"
	testFactionSHE       = "SHE"
	testPlayerName       = "TestPlayer"
	testFirebaseUIDLabel = "tst-firebase-uid-"

	playerSettingsPath               = "/api/v1/account/me/settings"
	onboardingNameSetSubscription    = "onboarding-name-set"
	onboardingFactionSetSubscription = "onboarding-faction-set"
)

var (
	sharedPg *postgrestest.Postgres

	serverBinaryPath        = filepath.Join(os.TempDir(), "account-server-test-"+uuid.NewString())
	buildServerBinaryOnce   sync.Once
	buildServerBinaryErr    error
	buildServerBinaryOutput []byte
)

func TestMain(m *testing.M) {
	exitCode := postgrestest.RunMain(m, &sharedPg,
		postgrestest.WithSchemaFile("db/schema.sql"),
	)
	_ = os.Remove(serverBinaryPath)
	os.Exit(exitCode)
}

func buildServerBinary(t *testing.T) {
	t.Helper()
	buildServerBinaryOnce.Do(func() {
		buildServerBinaryOutput, buildServerBinaryErr = exec.Command("go", "build", "-o", serverBinaryPath, ".").CombinedOutput()
	})
	require.NoError(t, buildServerBinaryErr, string(buildServerBinaryOutput))
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type serverSetup struct {
	env        map[string]string
	port       int
	projectID  string
	privateKey *rsa.PrivateKey
}

type runningServer struct {
	setup    *serverSetup
	cmd      *exec.Cmd
	stdout   *syncBuffer
	stderr   *syncBuffer
	exited   chan struct{}
	exitCode int
	client   *http.Client
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	require.NoError(t, err)
	return key
}

func encodePublicKeyPEM(t *testing.T, key *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func pickFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func signToken(t *testing.T, key *rsa.PrivateKey, playerID string) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": internalAuthKeyID})
	require.NoError(t, err)
	claims, err := json.Marshal(map[string]any{
		"iss": internalAuthIssuer,
		"sub": playerID,
		"exp": time.Now().Add(tokenValidDuration).Unix(),
	})
	require.NoError(t, err)

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func newServerSetup(t *testing.T) *serverSetup {
	t.Helper()
	database := sharedPg.NewDatabase(t)

	privateKey := generateRSAKey(t)
	port := pickFreePort(t)
	projectID := firestoretest.NewProjectID()

	return &serverSetup{
		env: map[string]string{
			"PORT":                        strconv.Itoa(port),
			"DATABASE_CONN":               database.ConnectionString,
			"GOOGLE_CLOUD_PROJECT_ID":     projectID,
			"INTERNAL_AUTH_PUBLIC_KEY":    encodePublicKeyPEM(t, &privateKey.PublicKey),
			"LOG_MODE":                    "production",
			"DATABASE_IAM_AUTH_ENABLED":   "false",
			firestoretest.EmulatorHostEnv: firestoretest.RequireEmulatorHost(t),
		},
		port:       port,
		projectID:  projectID,
		privateKey: privateKey,
	}
}

func (s *serverSetup) seedGameConfig(t *testing.T) {
	t.Helper()
	client := firestoretest.NewClient(t, s.projectID)
	firestoretest.SeedGameConfig(t, client, map[string]int64{
		"free_daily_battle_limit":    5,
		"premium_daily_battle_limit": 7,
		"initial_time_bank":          100,
		"exp_win":                    3,
		"exp_loss":                   2,
		"exp_draw":                   1,
		"exp_formula_coefficient":    10,
	})
}

func newSeededServerSetup(t *testing.T) *serverSetup {
	t.Helper()
	setup := newServerSetup(t)
	setup.seedGameConfig(t)
	return setup
}

func (s *serverSetup) start(t *testing.T) *runningServer {
	t.Helper()
	buildServerBinary(t)

	environment := make([]string, 0, len(s.env))
	for name, value := range s.env {
		environment = append(environment, name+"="+value)
	}

	srv := &runningServer{
		setup:  s,
		cmd:    exec.Command(serverBinaryPath),
		stdout: &syncBuffer{},
		stderr: &syncBuffer{},
		exited: make(chan struct{}),
		client: &http.Client{Timeout: httpClientTimeout},
	}
	srv.cmd.Env = environment
	srv.cmd.Stdout = srv.stdout
	srv.cmd.Stderr = srv.stderr
	require.NoError(t, srv.cmd.Start())

	go func() {
		_ = srv.cmd.Wait()
		srv.exitCode = srv.cmd.ProcessState.ExitCode()
		close(srv.exited)
	}()
	t.Cleanup(func() {
		_ = srv.cmd.Process.Kill()
		<-srv.exited
	})
	return srv
}

func (s *runningServer) waitForExit(t *testing.T) int {
	t.Helper()
	select {
	case <-s.exited:
	case <-time.After(processExitTimeout):
		t.Fatalf("プロセスが %s 以内に終了しなかった\nstdout: %s\nstderr: %s", processExitTimeout, s.stdout.String(), s.stderr.String())
	}
	return s.exitCode
}

func (s *runningServer) url(path string) string {
	return "http://127.0.0.1:" + strconv.Itoa(s.setup.port) + path
}

func (s *runningServer) requireHealthOK(t *testing.T) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := s.client.Get(s.url("/health"))
		require.NoError(c, err)
		defer resp.Body.Close()
		require.Equal(c, http.StatusOK, resp.StatusCode)

		var body apiaccount.HealthResponse
		require.NoError(c, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(c, "ok", body.Status)
	}, healthReadyTimeout, pollInterval)
}

func (s *runningServer) outputLines() []string {
	return strings.Split(s.stdout.String()+"\n"+s.stderr.String(), "\n")
}

type parsedLogLine struct {
	entry map[string]any
	err   error
}

func parseJSONLines(lines []string) []map[string]any {
	parsed := make([]parsedLogLine, len(lines))
	for i, line := range lines {
		parsed[i].err = json.Unmarshal([]byte(line), &parsed[i].entry)
	}
	parsed = slices.DeleteFunc(parsed, func(line parsedLogLine) bool { return line.err != nil })

	entries := make([]map[string]any, len(parsed))
	for i, line := range parsed {
		entries[i] = line.entry
	}
	return entries
}

func (s *runningServer) logEntries() []map[string]any {
	return parseJSONLines(s.outputLines())
}

func (s *runningServer) stdoutLogEntries() []map[string]any {
	return parseJSONLines(strings.Split(s.stdout.String(), "\n"))
}

func (s *runningServer) hasErrorLog(phrase string) bool {
	return slices.ContainsFunc(s.logEntries(), func(entry map[string]any) bool {
		errorText, _ := entry["error"].(string)
		return entry["severity"] == "ERROR" && strings.Contains(errorText, phrase)
	})
}

func (s *runningServer) requireInfoLogEventually(t *testing.T, message string, wantAttrs map[string]any) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		found := slices.ContainsFunc(s.logEntries(), func(entry map[string]any) bool {
			matched := entry["severity"] == "INFO" && entry["message"] == message
			for name, value := range wantAttrs {
				matched = matched && entry[name] == value
			}
			return matched
		})
		assert.True(c, found, "Info ログ %q が出ていない\nstdout: %s\nstderr: %s", message, s.stdout.String(), s.stderr.String())
	}, logAppearTimeout, pollInterval)
}

func (s *runningServer) requireErrorLogEventually(t *testing.T, phrase string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, s.hasErrorLog(phrase), "エラーログに %q が出ていない\nstdout: %s\nstderr: %s", phrase, s.stdout.String(), s.stderr.String())
	}, logAppearTimeout, pollInterval)
}

func (s *runningServer) send(t *testing.T, req *http.Request) (int, []byte) {
	t.Helper()
	resp, err := s.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, respBody
}

func (s *runningServer) newJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(method, s.url(path), bytes.NewReader(encoded))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (s *runningServer) getWithToken(t *testing.T, path, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.url(path), nil)
	require.NoError(t, err)
	req.Header.Set(internalAuthHeader, token)
	return s.send(t, req)
}

func (s *runningServer) postJSON(t *testing.T, path string, body any) (int, []byte) {
	t.Helper()
	return s.send(t, s.newJSONRequest(t, http.MethodPost, path, body))
}

func (s *runningServer) putJSONWithToken(t *testing.T, path, token string, body any) (int, []byte) {
	t.Helper()
	req := s.newJSONRequest(t, http.MethodPut, path, body)
	req.Header.Set(internalAuthHeader, token)
	return s.send(t, req)
}

func (s *runningServer) registerPlayer(t *testing.T) string {
	t.Helper()
	status, body := s.postJSON(t, "/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: testFirebaseUIDLabel + uuid.NewString()})
	require.Equal(t, http.StatusCreated, status, string(body))
	var player apiaccount.PlayerResponse
	require.NoError(t, json.Unmarshal(body, &player))
	return player.PlayerID
}

func (s *runningServer) fetchPlayer(t *testing.T, playerID string) apiaccount.PlayerResponse {
	t.Helper()
	status, body := s.getWithToken(t, "/api/v1/account/me", signToken(t, s.setup.privateKey, playerID))
	require.Equal(t, http.StatusOK, status, string(body))
	var player apiaccount.PlayerResponse
	require.NoError(t, json.Unmarshal(body, &player))
	return player
}

func (s *runningServer) fetchFactions(t *testing.T, playerID string) []string {
	t.Helper()
	status, body := s.getWithToken(t, "/api/v1/account/me/factions", signToken(t, s.setup.privateKey, playerID))
	require.Equal(t, http.StatusOK, status, string(body))
	var listing apiaccount.FactionListing
	require.NoError(t, json.Unmarshal(body, &listing))
	return listing.Factions
}

func (s *runningServer) sendEvent(t *testing.T, subscription string, event any) (int, []byte) {
	t.Helper()
	data, err := json.Marshal(event)
	require.NoError(t, err)
	envelope := pubsubpushtest.BuildEventEnvelope(t, string(data), uuid.NewString())
	return s.postJSON(t, "/internal/v1/pubsub/"+subscription, json.RawMessage(envelope))
}

func (s *runningServer) pushEvent(t *testing.T, subscription string, event any) {
	t.Helper()
	status, body := s.sendEvent(t, subscription, event)
	require.GreaterOrEqual(t, status, http.StatusOK, string(body))
	require.Less(t, status, http.StatusMultipleChoices, string(body))
}

func (s *runningServer) pushOnboardingNameSet(t *testing.T, playerID string) {
	t.Helper()
	s.pushEvent(t, onboardingNameSetSubscription, apiscenario.OnboardingNameSetEvent{
		EventType: "onboarding_name_set",
		EventID:   uuid.NewString(),
		Timestamp: time.Now().UTC(),
		PlayerID:  playerID,
		Name:      testPlayerName,
	})
}

func newOnboardingFactionSetEvent(playerID string) apiscenario.OnboardingFactionSetEvent {
	return apiscenario.OnboardingFactionSetEvent{
		EventType:        "onboarding_faction_set",
		EventID:          uuid.NewString(),
		Timestamp:        time.Now().UTC(),
		PlayerID:         playerID,
		InitialFactionID: testFactionSHE,
	}
}

func (s *runningServer) pushOnboardingFactionSet(t *testing.T, playerID string) {
	t.Helper()
	s.pushEvent(t, onboardingFactionSetSubscription, newOnboardingFactionSetEvent(playerID))
}

func (s *runningServer) sendOnboardingFactionSet(t *testing.T, playerID string) (int, []byte) {
	t.Helper()
	return s.sendEvent(t, onboardingFactionSetSubscription, newOnboardingFactionSetEvent(playerID))
}

func (s *runningServer) updateBgmVolume(t *testing.T, playerID string, bgmVolume int64) (int, []byte) {
	t.Helper()
	return s.putJSONWithToken(t, playerSettingsPath, signToken(t, s.setup.privateKey, playerID), apiaccount.UpdateSettingsRequest{BgmVolume: &bgmVolume})
}

func (s *runningServer) fetchSettings(t *testing.T, playerID string) apiaccount.PlayerSettingsResponse {
	t.Helper()
	status, body := s.getWithToken(t, playerSettingsPath, signToken(t, s.setup.privateKey, playerID))
	require.Equal(t, http.StatusOK, status, string(body))
	var settings apiaccount.PlayerSettingsResponse
	require.NoError(t, json.Unmarshal(body, &settings))
	return settings
}

func newSetupWithoutDatabaseConn(t *testing.T) *serverSetup {
	t.Helper()
	setup := newSeededServerSetup(t)
	delete(setup.env, "DATABASE_CONN")
	return setup
}

func startHealthyServerWithPlayer(t *testing.T) (*runningServer, string) {
	t.Helper()
	srv := newSeededServerSetup(t).start(t)
	srv.requireHealthOK(t)
	return srv, srv.registerPlayer(t)
}

func TestRunServer(t *testing.T) {
	t.Run("[サービス起動]アカウントサービスの起動と停止", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("必須の環境変数がすべて有効でgame_configが投入済みのとき", func(t *testing.T) {
				t.Run("起動すると、PORTで指定したポートの／healthの応答のstatusはokになる", func(t *testing.T) {
					srv := newSeededServerSetup(t).start(t)
					srv.requireHealthOK(t)
				})

				t.Run("起動すると、PORTで指定したポートの待受アドレスを示すInfoログが標準出力または標準エラーに出る", func(t *testing.T) {
					setup := newSeededServerSetup(t)
					srv := setup.start(t)
					srv.requireInfoLogEventually(t, "account starting", map[string]any{"addr": ":" + strconv.Itoa(setup.port)})
				})

				t.Run("起動したプロセスに停止要求を送ると", func(t *testing.T) {
					tests := []struct {
						name   string
						signal syscall.Signal
					}{
						{"停止要求がSIGINTのとき、終了コード0で終了する", syscall.SIGINT},
						{"停止要求がSIGTERMのとき、終了コード0で終了する", syscall.SIGTERM},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							srv := newSeededServerSetup(t).start(t)
							srv.requireHealthOK(t)
							require.NoError(t, srv.cmd.Process.Signal(tt.signal))
							assert.Equal(t, 0, srv.waitForExit(t))
						})
					}

					t.Run("停止要求がSIGTERMのとき、停止要求を受けたことを示すInfoログが標準出力または標準エラーに出る", func(t *testing.T) {
						srv := newSeededServerSetup(t).start(t)
						srv.requireHealthOK(t)
						require.NoError(t, srv.cmd.Process.Signal(syscall.SIGTERM))
						srv.waitForExit(t)
						srv.requireInfoLogEventually(t, "shutdown requested", nil)
					})
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("起動に失敗するとき", func(t *testing.T) {
				t.Run("DATABASE_CONNが未設定のとき", func(t *testing.T) {
					t.Run("起動すると、DATABASE_CONNが必須であることを示すエラーログが標準出力または標準エラーに出る", func(t *testing.T) {
						srv := newSetupWithoutDatabaseConn(t).start(t)
						srv.waitForExit(t)
						assert.True(t, srv.hasErrorLog("DATABASE_CONN is required"), "stdout: %s\nstderr: %s", srv.stdout.String(), srv.stderr.String())
					})

					t.Run("起動すると、終了コード1で終了する", func(t *testing.T) {
						srv := newSetupWithoutDatabaseConn(t).start(t)
						assert.Equal(t, 1, srv.waitForExit(t))
					})
				})

				t.Run("DATABASE_CONNのポートがnot-a-portのとき、データベース接続プールの構築に失敗したことを示すエラーログが標準出力または標準エラーに出る", func(t *testing.T) {
					setup := newSeededServerSetup(t)
					setup.env["DATABASE_CONN"] = "postgres://tst-user:tst-pass@tst-host:not-a-port/tst-db"
					srv := setup.start(t)
					srv.waitForExit(t)
					assert.True(t, srv.hasErrorLog("new database pool"), "stdout: %s\nstderr: %s", srv.stdout.String(), srv.stderr.String())
				})

				t.Run("game_configが未投入のとき、game_configの検証に失敗したことを示すエラーログが標準出力または標準エラーに出る", func(t *testing.T) {
					srv := newServerSetup(t).start(t)
					srv.waitForExit(t)
					assert.True(t, srv.hasErrorLog("validate game_config"), "stdout: %s\nstderr: %s", srv.stdout.String(), srv.stderr.String())
				})

				t.Run("INTERNAL_AUTH_PUBLIC_KEYがtst-not-pemのとき、INTERNAL_AUTH_PUBLIC_KEYが不正であることを示すエラーログが標準出力または標準エラーに出る", func(t *testing.T) {
					setup := newSeededServerSetup(t)
					setup.env["INTERNAL_AUTH_PUBLIC_KEY"] = "tst-not-pem"
					srv := setup.start(t)
					srv.waitForExit(t)
					assert.True(t, srv.hasErrorLog("INTERNAL_AUTH_PUBLIC_KEY is invalid"), "stdout: %s\nstderr: %s", srv.stdout.String(), srv.stderr.String())
				})

				t.Run("PORTで指定したポートを別のプロセスが使用中のとき、HTTPサーバーの待受に失敗したことを示すエラーログが標準出力または標準エラーに出る", func(t *testing.T) {
					setup := newSeededServerSetup(t)
					occupier, err := net.Listen("tcp", ":0")
					require.NoError(t, err)
					t.Cleanup(func() { _ = occupier.Close() })
					setup.env["PORT"] = strconv.Itoa(occupier.Addr().(*net.TCPAddr).Port)
					srv := setup.start(t)
					srv.waitForExit(t)
					assert.True(t, srv.hasErrorLog("http server"), "stdout: %s\nstderr: %s", srv.stdout.String(), srv.stderr.String())
				})

				t.Run("起動すると", func(t *testing.T) {
					cases := []struct {
						name  string
						setup func(t *testing.T) *serverSetup
					}{
						{
							name:  "DATABASE_CONNが未設定のとき、起動失敗を示すログが、severityがERRORのJSONとして標準出力に出る",
							setup: newSetupWithoutDatabaseConn,
						},
						{
							name:  "LOG_MODEがproductionでgame_configが未投入のとき、起動失敗を示すログが、severityがERRORのJSONとして標準出力に出る",
							setup: newServerSetup,
						},
					}
					for _, tc := range cases {
						t.Run(tc.name, func(t *testing.T) {
							srv := tc.setup(t).start(t)
							srv.waitForExit(t)

							found := slices.ContainsFunc(srv.stdoutLogEntries(), func(entry map[string]any) bool {
								return entry["message"] == "account fatal" && entry["severity"] == "ERROR"
							})
							assert.True(t, found, "stdout: %s\nstderr: %s", srv.stdout.String(), srv.stderr.String())
						})
					}
				})

				t.Run("game_configが未投入で起動に失敗したあと、game_configを投入して再び起動すると、起動に成功して／healthの応答のstatusはokになる", func(t *testing.T) {
					setup := newServerSetup(t)
					failed := setup.start(t)
					require.Equal(t, 1, failed.waitForExit(t))

					setup.seedGameConfig(t)
					restarted := setup.start(t)
					restarted.requireHealthOK(t)
				})
			})
		})
	})
}

func TestRunServerInternalAuth(t *testing.T) {
	t.Run("[内部API認証]内部認証トークンの検証", func(t *testing.T) {
		t.Run("必須の環境変数がすべて有効でgame_configが投入済みのとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				t.Run("登録済みのプレイヤーのplayer_idを、INTERNAL_AUTH_PUBLIC_KEYと対の秘密鍵で署名したトークンで／api／v1／account／meに問い合わせると、登録済みのプレイヤーのplayer_idが返る", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					player := srv.fetchPlayer(t, playerID)
					assert.Equal(t, playerID, player.PlayerID)
				})
			})

			t.Run("異常系", func(t *testing.T) {
				t.Run("認証に失敗するとき", func(t *testing.T) {
					t.Run("INTERNAL_AUTH_PUBLIC_KEYと対でない秘密鍵で署名したトークンで／api／v1／account／meに問い合わせると、トークンが無効であることを示すステータス401が返る", func(t *testing.T) {
						srv, playerID := startHealthyServerWithPlayer(t)
						status, body := srv.getWithToken(t, "/api/v1/account/me", signToken(t, generateRSAKey(t), playerID))
						assert.Equal(t, http.StatusUnauthorized, status)
						var errorBody apiaccount.ErrorResponse
						require.NoError(t, json.Unmarshal(body, &errorBody))
						assert.Equal(t, "invalid internal auth token", errorBody.Error)
					})

					t.Run("対でない秘密鍵で署名したトークンを拒否したあと、INTERNAL_AUTH_PUBLIC_KEYと対の秘密鍵で署名したトークンで再び問い合わせると、認証に成功して／api／v1／account／meが登録済みのプレイヤーのplayer_idを返す", func(t *testing.T) {
						srv, playerID := startHealthyServerWithPlayer(t)
						rejectedStatus, _ := srv.getWithToken(t, "/api/v1/account/me", signToken(t, generateRSAKey(t), playerID))
						require.Equal(t, http.StatusUnauthorized, rejectedStatus)

						player := srv.fetchPlayer(t, playerID)
						assert.Equal(t, playerID, player.PlayerID)
					})
				})
			})
		})
	})
}

func TestRunServerPlayerSettings(t *testing.T) {
	t.Run("[プレイヤー設定]設定の更新", func(t *testing.T) {
		t.Run("必須の環境変数がすべて有効でgame_configが投入済みのとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				t.Run("登録済みのプレイヤーの設定をbgm_volume80で更新すると、更新の応答のbgm_volumeは80になる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					status, body := srv.updateBgmVolume(t, playerID, 80)
					require.Equal(t, http.StatusOK, status, string(body))
					var settings apiaccount.PlayerSettingsResponse
					require.NoError(t, json.Unmarshal(body, &settings))
					assert.Equal(t, int64(80), settings.BgmVolume)
				})
			})

			t.Run("異常系", func(t *testing.T) {
				t.Run("登録済みのプレイヤーの設定をbgm_volume101で更新すると、bgm_volumeが範囲外であることを示すステータス400が返る", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					status, body := srv.updateBgmVolume(t, playerID, 101)
					assert.Equal(t, http.StatusBadRequest, status)
					var errorBody apiaccount.ErrorResponse
					require.NoError(t, json.Unmarshal(body, &errorBody))
					assert.Equal(t, "bgm_volume is invalid", errorBody.Error)
				})

				t.Run("bgm_volume101での更新が拒否されたあと、bgm_volume80で同じ設定を更新すると、更新に成功して取得した設定のbgm_volumeは80になる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					rejectedStatus, _ := srv.updateBgmVolume(t, playerID, 101)
					require.Equal(t, http.StatusBadRequest, rejectedStatus)

					status, body := srv.updateBgmVolume(t, playerID, 80)
					require.Equal(t, http.StatusOK, status, string(body))
					assert.Equal(t, int64(80), srv.fetchSettings(t, playerID).BgmVolume)
				})
			})
		})
	})
}

func TestRunServerEventReception(t *testing.T) {
	t.Run("[イベント受信]受信したイベントのプレイヤー情報への反映", func(t *testing.T) {
		t.Run("必須の環境変数がすべて有効でgame_configが投入済みで、登録直後のプレイヤーがいるとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				t.Run("is_premiumがtrueのプレミアム状態更新イベントをpremium-updatedの送信先に送ると、／api／v1／account／meの応答のis_premiumはtrueになる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					srv.pushEvent(t, "premium-updated", apishop.PremiumUpdatedEvent{
						EventType: "premium_updated",
						EventID:   uuid.NewString(),
						Timestamp: time.Now().UTC(),
						PlayerID:  playerID,
						IsPremium: true,
						Source:    "shop",
					})
					assert.True(t, srv.fetchPlayer(t, playerID).IsPremium)
				})

				t.Run("SHEの陣営獲得イベントをfaction-acquiredの送信先に送ると、／api／v1／account／me／factionsの応答の陣営一覧にSHEが含まれる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					srv.pushEvent(t, "faction-acquired", apishop.FactionAcquiredEvent{
						EventType: "faction_acquired",
						EventID:   uuid.NewString(),
						Timestamp: time.Now().UTC(),
						PlayerID:  playerID,
						Faction:   testFactionSHE,
					})
					assert.Contains(t, srv.fetchFactions(t, playerID), "SHE")
				})

				t.Run("オンボーディング表示名設定イベントをonboarding-name-setの送信先に送ると、／api／v1／account／meの応答のonboarding_statusはname_setになる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					srv.pushOnboardingNameSet(t, playerID)
					assert.Equal(t, "name_set", string(srv.fetchPlayer(t, playerID).OnboardingStatus))
				})

				t.Run("オンボーディング表示名設定イベントを送ったあと、オンボーディング陣営設定イベントをonboarding-faction-setの送信先に送ると、／api／v1／account／meの応答のonboarding_statusはfaction_setになる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					srv.pushOnboardingNameSet(t, playerID)
					srv.pushOnboardingFactionSet(t, playerID)
					assert.Equal(t, "faction_set", string(srv.fetchPlayer(t, playerID).OnboardingStatus))
				})

				t.Run("オンボーディング表示名設定イベントとオンボーディング陣営設定イベントを順に送ったあと、オンボーディング完了イベントをplayer-onboardedの送信先に送ると、／api／v1／account／meの応答のonboarding_statusはcompletedになる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					srv.pushOnboardingNameSet(t, playerID)
					srv.pushOnboardingFactionSet(t, playerID)
					srv.pushEvent(t, "player-onboarded", apiscenario.PlayerOnboardedEvent{
						EventType:        "player_onboarded",
						EventID:          uuid.NewString(),
						Timestamp:        time.Now().UTC(),
						PlayerID:         playerID,
						InitialFactionID: testFactionSHE,
					})
					assert.Equal(t, "completed", string(srv.fetchPlayer(t, playerID).OnboardingStatus))
				})
			})

			t.Run("異常系", func(t *testing.T) {
				t.Run("オンボーディング表示名設定イベントを送らずに、オンボーディング陣営設定イベントをonboarding-faction-setの送信先に送ると", func(t *testing.T) {
					t.Run("処理失敗を示すステータス500が返る", func(t *testing.T) {
						srv, playerID := startHealthyServerWithPlayer(t)
						status, body := srv.sendOnboardingFactionSet(t, playerID)
						assert.Equal(t, http.StatusInternalServerError, status)
						var errorBody apiaccount.ErrorResponse
						require.NoError(t, json.Unmarshal(body, &errorBody))
						assert.Equal(t, "handler failed", errorBody.Error)
					})

					t.Run("オンボーディング進行状態が段階を飛ばしていることを示すエラーログが標準出力または標準エラーに出る", func(t *testing.T) {
						srv, playerID := startHealthyServerWithPlayer(t)
						srv.sendOnboardingFactionSet(t, playerID)
						srv.requireErrorLogEventually(t, "onboarding status skipped a stage")
					})
				})

				t.Run("オンボーディング表示名設定イベントを送らずに送ったオンボーディング陣営設定イベントが500になったあと、オンボーディング表示名設定イベントを送り、続けてオンボーディング陣営設定イベントを再び送ると、／api／v1／account／meの応答のonboarding_statusはfaction_setになる", func(t *testing.T) {
					srv, playerID := startHealthyServerWithPlayer(t)
					rejectedStatus, _ := srv.sendOnboardingFactionSet(t, playerID)
					require.Equal(t, http.StatusInternalServerError, rejectedStatus)

					srv.pushOnboardingNameSet(t, playerID)
					srv.pushOnboardingFactionSet(t, playerID)
					assert.Equal(t, "faction_set", string(srv.fetchPlayer(t, playerID).OnboardingStatus))
				})
			})
		})
	})
}
