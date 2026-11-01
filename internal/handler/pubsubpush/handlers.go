package pubsubpush

import (
	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
)

// Handlers はイベント別の push 受け口をまとめたものである。
type Handlers struct {
	FactionAcquired      *EventHandler[apishop.FactionAcquiredEvent]
	PremiumUpdated       *EventHandler[PremiumUpdatedEvent]
	PlayerOnboarded      *EventHandler[apiscenario.PlayerOnboardedEvent]
	OnboardingNameSet    *EventHandler[apiscenario.OnboardingNameSetEvent]
	OnboardingFactionSet *EventHandler[apiscenario.OnboardingFactionSetEvent]
}
