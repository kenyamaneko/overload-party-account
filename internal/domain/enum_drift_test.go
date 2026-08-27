package domain_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

type openapiSchemaDoc struct {
	Components struct {
		Schemas struct {
			OnboardingStatus struct {
				Enum []string `yaml:"enum"`
			} `yaml:"OnboardingStatus"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

func TestOnboardingStatusEnumMatchesRESTContract(t *testing.T) {
	t.Run("[オンボーディング状態]REST契約とのオンボーディング状態値の一致", func(t *testing.T) {
		t.Run("domainパッケージが定義するオンボーディング状態(not_started/name_set/faction_set/completed)の集合は、openapi.yamlのOnboardingStatus enumと完全に一致する", func(t *testing.T) {
			raw, err := os.ReadFile("../../data/openapi.yaml")
			require.NoError(t, err)

			var doc openapiSchemaDoc
			require.NoError(t, yaml.Unmarshal(raw, &doc))

			domainValues := []string{
				domain.OnboardingStatusNotStarted,
				domain.OnboardingStatusNameSet,
				domain.OnboardingStatusFactionSet,
				domain.OnboardingStatusCompleted,
			}

			require.ElementsMatch(t, domainValues, doc.Components.Schemas.OnboardingStatus.Enum)
		})
	})
}
