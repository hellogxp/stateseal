#!/usr/bin/env python3
"""Reject accidental CJK copy in canonical English documentation.

Localized language names in language selectors and the localization matrix are
the only intentional exceptions. Protocol values and historical evidence live
in explicitly localized files and are outside this canonical-English scan.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent

# Han, Hiragana, Katakana, Hangul Jamo, and Hangul syllables. These ranges catch
# the accidental Chinese example that motivated the guard and the equivalent
# class of cross-locale copy/paste errors.
LOCALIZED_SCRIPT = re.compile(
    "["
    "\u3400-\u4dbf"
    "\u4e00-\u9fff"
    "\uf900-\ufaff"
    "\u3040-\u30ff"
    "\u1100-\u11ff"
    "\u3130-\u318f"
    "\uac00-\ud7af"
    "]"
)

ALLOWED_LANGUAGE_NAMES = ("简体中文", "日本語", "한국어")


def english_sources() -> list[Path]:
    docs = sorted(
        path
        for path in (ROOT / "docs").glob("*.md")
        if not path.name.endswith(".zh-CN.md")
    )
    return [
        ROOT / "README.md",
        *docs,
        ROOT / "internal/ui/web/locales/en.json",
    ]


def remove_declared_language_names(path: Path, line: str) -> str:
    if path == ROOT / "README.md" or path == ROOT / "docs/index.md":
        for name in ALLOWED_LANGUAGE_NAMES:
            line = line.replace(name, "")
        return line

    if path == ROOT / "docs/localization.md" and line.startswith("| `"):
        for name in ALLOWED_LANGUAGE_NAMES:
            line = line.replace(name, "")
        return line

    return line


def main() -> int:
    failures: list[str] = []
    for path in english_sources():
        if not path.is_file():
            failures.append(f"missing canonical English source: {path.relative_to(ROOT)}")
            continue

        for line_number, original in enumerate(
            path.read_text(encoding="utf-8").splitlines(), start=1
        ):
            inspected = remove_declared_language_names(path, original)
            match = LOCALIZED_SCRIPT.search(inspected)
            if match:
                relative = path.relative_to(ROOT)
                failures.append(
                    f"{relative}:{line_number}: unexpected localized script "
                    f"{match.group()!r}: {original.strip()}"
                )

    if failures:
        print("Canonical English documentation contains cross-locale copy:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1

    print("Canonical English documentation language check passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
