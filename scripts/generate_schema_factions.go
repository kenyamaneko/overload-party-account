//go:build ignore

// generate_schema_factions は、db/schema.sql の陣営の CHECK 制約の値を、
// 陣営の SSoT (common の factions.yaml) から生成された game-design-constants の陣営一覧に合わせて書き換える。
// -check を付けると書き換えず、schema.sql が最新かどうかだけを確かめる (最新でなければ終了コード 1)。
//
// 使い方 (リポジトリのルートで実行する):
//
//	go run scripts/generate_schema_factions.go
//	go run scripts/generate_schema_factions.go -check
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"
)

const (
	schemaPath     = "db/schema.sql"
	schemaFileMode = 0o644
)

// factionCheckPattern は player_factions.faction の CHECK 制約のうち、値の一覧を含む部分に一致する。
var factionCheckPattern = regexp.MustCompile(`CHECK \(faction IN \([^)]*\)\)`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate_schema_factions:", err)
		os.Exit(1)
	}
}

func run() error {
	isCheckOnly := flag.Bool("check", false, "schema.sql を書き換えず、最新かどうかだけを確かめる")
	flag.Parse()

	current, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", schemaPath, err)
	}
	if n := len(factionCheckPattern.FindAll(current, -1)); n != 1 {
		return fmt.Errorf("%s にある陣営の CHECK 制約が 1 つではない (%d 個)", schemaPath, n)
	}

	generated := factionCheckPattern.ReplaceAllLiteral(current, []byte(buildFactionCheck()))
	if string(generated) == string(current) {
		return nil
	}
	if *isCheckOnly {
		return fmt.Errorf("%s の陣営の CHECK 制約が game-design-constants と食い違っている。go run scripts/generate_schema_factions.go で再生成する", schemaPath)
	}
	if err := os.WriteFile(schemaPath, generated, schemaFileMode); err != nil {
		return fmt.Errorf("write %s: %w", schemaPath, err)
	}
	return nil
}

// buildFactionCheck は、全陣営を SortOrder の順に並べた CHECK 制約を返す。
func buildFactionCheck() string {
	quoted := make([]string, 0, len(gamedesign.FactionsMetadata))
	for _, faction := range gamedesign.FactionsMetadata {
		quoted = append(quoted, "'"+faction.ID+"'")
	}
	return "CHECK (faction IN (" + strings.Join(quoted, ", ") + "))"
}
