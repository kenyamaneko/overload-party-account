package postgrestest

import (
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	readDatabaseTimeQuery = "SELECT now()"

	// SQLStateForeignKeyViolation は外部キー制約違反の SQLSTATE。
	SQLStateForeignKeyViolation = "23503"
	// SQLStateUniqueViolation は一意制約違反の SQLSTATE。
	SQLStateUniqueViolation = "23505"
)

// ToUTCTimes は時刻のポインタを、未設定なら空の一覧に、設定済みなら UTC の 1 件の一覧にする。
func ToUTCTimes(p *time.Time) []time.Time {
	present := slices.DeleteFunc([]*time.Time{p}, func(v *time.Time) bool { return v == nil })
	times := make([]time.Time, 0, len(present))
	for _, v := range present {
		times = append(times, v.UTC())
	}
	return times
}

// ReadDatabaseTime はケース専用のデータベースの現在時刻を返す。
func ReadDatabaseTime(t *testing.T, pool *pgxpool.Pool) time.Time {
	t.Helper()
	var now time.Time
	require.NoError(t, pool.QueryRow(t.Context(), readDatabaseTimeQuery).Scan(&now))
	return now
}

// RequirePgError はエラーが指定した SQLSTATE の PostgreSQL のエラーであることを確かめ、そのエラーを返す。
func RequirePgError(t *testing.T, err error, sqlState string) *pgconn.PgError {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, sqlState, pgErr.Code)
	return pgErr
}

// FilterFailures はエラーの一覧から、nil でないものだけを返す。
func FilterFailures(errs []error) []error {
	return slices.DeleteFunc(slices.Clone(errs), func(err error) bool { return err == nil })
}

// FindSucceededIndex はエラーの一覧で最初に nil の位置を返す。nil が無ければ -1 を返す。
func FindSucceededIndex(errs []error) int {
	return slices.IndexFunc(errs, func(err error) bool { return err == nil })
}
