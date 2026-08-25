//go:build integration

package rest_test

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

var sharedPg *postgrestest.Postgres

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(postgrestest.RunMain(m, &sharedPg,
		postgrestest.WithSchemaFile("db/schema.sql"),
		postgrestest.WithSchema("account"),
	))
}
