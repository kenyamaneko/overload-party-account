// data/openapi.yaml から openapi-typescript で生成された components / paths の re-export。
// 利用者は本ファイルが re-export する schema 型を import すること。

import type { components, paths } from "./openapi.gen";

export type { components, paths };

type Schemas = components["schemas"];

export type HealthResponse = Schemas["HealthResponse"];
export type PlayerResponse = Schemas["PlayerResponse"];
export type PlayerSettingsResponse = Schemas["PlayerSettingsResponse"];
export type BattleLimitResponse = Schemas["BattleLimitResponse"];
export type FactionListing = Schemas["FactionListing"];
export type RegisterRequest = Schemas["RegisterRequest"];
export type LoginRequest = Schemas["LoginRequest"];
export type UpdateNameRequest = Schemas["UpdateNameRequest"];
export type ValidateNameForOnboardingRequest = Schemas["ValidateNameForOnboardingRequest"];
export type UpdatePremiumRequest = Schemas["UpdatePremiumRequest"];
export type AddExpRequest = Schemas["AddExpRequest"];
export type AwardGameExpRequest = Schemas["AwardGameExpRequest"];
export type FactionGrantRequest = Schemas["FactionGrantRequest"];
export type SelectInitialFactionRequest = Schemas["SelectInitialFactionRequest"];
export type UpdateSettingsRequest = Schemas["UpdateSettingsRequest"];
export type OnboardingStatus = Schemas["OnboardingStatus"];
