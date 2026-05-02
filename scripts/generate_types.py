#!/usr/bin/env python3
"""data/models.yaml から packages/api-account / internal/domain に Go 型を生成する.

共通基盤 `overload-party-codegen-tools` の `CodegenRunner` を使う。account では
target ごとに付与するタグを切り替える (api → json, domain → db) ため、
`GoTarget.emit_tags` で吸収する。`target: both` で両方に出すこともできる。

実行: python3 scripts/generate_types.py
"""

from __future__ import annotations

import sys
from pathlib import Path

from codegen_tools import CodegenRunner, GoStyle, GoTarget

REPO_ROOT = Path(__file__).resolve().parent.parent
MODELS_YAML = REPO_ROOT / "data" / "models.yaml"


def main() -> int:
    runner = CodegenRunner(
        models_yaml=MODELS_YAML,
        repo_root=REPO_ROOT,
        targets={
            "api": GoTarget(
                out_dir=REPO_ROOT / "packages" / "api-account",
                package="apiaccount",
                emit_tags=("json",),
            ),
            "domain": GoTarget(
                out_dir=REPO_ROOT / "internal" / "domain",
                package="domain",
                emit_tags=("db",),
            ),
        },
        style=GoStyle(),
        single_target_field="target",
        multi_target_field=None,
        all_targets_keyword="both",
        trailing_blank_line=True,
    )
    return runner.run()


if __name__ == "__main__":
    sys.exit(main())
