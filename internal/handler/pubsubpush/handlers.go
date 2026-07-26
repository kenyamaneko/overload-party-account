package pubsubpush

// Handlers は account が公開する 5 つの push 受け口をイベント別にまとめたもの。
type Handlers struct {
	FactionAcquired      *EventHandler
	PremiumUpdated       *EventHandler
	PlayerOnboarded      *EventHandler
	OnboardingNameSet    *EventHandler
	OnboardingFactionSet *EventHandler
}
