package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/config"
)

func readAllAndClose(t *testing.T, reader *os.File) string {
	t.Helper()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return string(data)
}

func captureLogOutput(t *testing.T, mode config.LogMode, emit func()) (stdout, stderr string) {
	t.Helper()

	previousLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	previousStdout, previousStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	setupErr := setupLogger(mode)
	emit()
	os.Stdout, os.Stderr = previousStdout, previousStderr

	require.NoError(t, setupErr)
	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())

	return readAllAndClose(t, stdoutReader), readAllAndClose(t, stderrReader)
}

func captureProductionJSON(t *testing.T, emit func()) map[string]any {
	t.Helper()
	stdout, _ := captureLogOutput(t, config.LogModeProduction, emit)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &entry))
	return entry
}

func TestSetupLogger(t *testing.T) {
	t.Run("[ログ出力]LOG_MODEに応じたログ出力形式の切り替え", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("LOG_MODEがproductionのとき", func(t *testing.T) {
				t.Run("Infoログを出力すると、標準出力に出るJSONのseverityはINFOになる", func(t *testing.T) {
					entry := captureProductionJSON(t, func() { slog.Info("tst-info-message") })
					assert.Equal(t, "INFO", entry["severity"])
				})

				t.Run("Warnログを出力すると、標準出力に出るJSONのseverityはWARNINGになる", func(t *testing.T) {
					entry := captureProductionJSON(t, func() { slog.Warn("tst-warn-message") })
					assert.Equal(t, "WARNING", entry["severity"])
				})

				t.Run("Errorログを出力すると、標準出力に出るJSONのseverityはERRORになる", func(t *testing.T) {
					entry := captureProductionJSON(t, func() { slog.Error("tst-error-message") })
					assert.Equal(t, "ERROR", entry["severity"])
				})

				t.Run("文言がtst-output-messageのログを出力すると、標準出力に出るJSONのmessageはtst-output-messageになる", func(t *testing.T) {
					entry := captureProductionJSON(t, func() { slog.Info("tst-output-message") })
					assert.Equal(t, "tst-output-message", entry["message"])
				})

				t.Run("ログを出力すると、標準出力に出るJSONのserviceはaccountになる", func(t *testing.T) {
					entry := captureProductionJSON(t, func() { slog.Info("tst-service-message") })
					assert.Equal(t, "account", entry["service"])
				})

				t.Run("Debugログを出力すると、標準出力にも標準エラーにも何も出ない", func(t *testing.T) {
					stdout, stderr := captureLogOutput(t, config.LogModeProduction, func() { slog.Debug("tst-debug-message") })
					assert.Empty(t, stdout)
					assert.Empty(t, stderr)
				})
			})

			t.Run("LOG_MODEがlocalのとき", func(t *testing.T) {
				t.Run("文言がtst-local-debug-messageのDebugログを出力すると、標準エラーにtst-local-debug-messageが出る", func(t *testing.T) {
					_, stderr := captureLogOutput(t, config.LogModeLocal, func() { slog.Debug("tst-local-debug-message") })
					assert.Contains(t, stderr, "tst-local-debug-message")
				})

				t.Run("ログを出力すると、標準エラーの出力にservice=accountが含まれる", func(t *testing.T) {
					_, stderr := captureLogOutput(t, config.LogModeLocal, func() { slog.Info("tst-local-service-message") })
					assert.Contains(t, stderr, "service=account")
				})
			})
		})
	})
}
