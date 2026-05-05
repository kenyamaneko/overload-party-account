// Package domain は account のエンティティと、生成 struct に手書きで昇格させた
// 業務ロジック (level / name / onboarding_status の判定・生成) を保持する。
// 振る舞いを domain に置く昇格基準は docs/ARCHITECTURE.md「ドメイン層の責務」を参照。
package domain
