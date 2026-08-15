#!/usr/bin/env python3
"""STY-006: audit the 44 style->class replacements made by STY-005.

For each file, extract every `style={{...}}` from the d5903cf baseline with
its element context (the enclosing JSX tag), then list every className in the
current file, and dump the matching rule bodies from globals.css. The pairing
(same element, same value) is judged by the report author from the three
columns; this script only produces the mechanical facts.
"""
import re
import subprocess
import sys

FILES = [
    "web/app/operating-pulse/page.tsx",
    "web/app/store-360/page.tsx",
    "web/app/scenario-workbench/page.tsx",
    "web/app/contracts/[id]/page.tsx",
]

GLOBALS = "web/app/globals.css"


def baseline_source(path):
    return subprocess.run(
        ["git", "show", f"d5903cf:{path}"], capture_output=True, text=True, check=True
    ).stdout


def read(path):
    with open(path, encoding="utf-8") as fh:
        return fh.read()


def enclosing_tag(lines, index):
    """Walk up from `index` to find the nearest JSX open tag line."""
    for i in range(index, -1, -1):
        m = re.search(r"<([A-Za-z][\w.]*)\b", lines[i])
        if m:
            return m.group(1), i + 1
    return "?", 0


def extract_styles(src):
    out = []
    lines = src.splitlines()
    for i, line in enumerate(lines):
        for m in re.finditer(r"style=\{\{(.*?)\}\}", line):
            out.append((i + 1, enclosing_tag(lines, i), m.group(1)))
    return out


def extract_classes(src):
    out = []
    lines = src.splitlines()
    for i, line in enumerate(lines):
        for m in re.finditer(r'className="([^"]+)"', line):
            for cls in m.group(1).split():
                if not cls.startswith(("ant-", "css-")):
                    out.append((i + 1, cls))
    return out


def rule_body(css, cls):
    """Return the rule body whose selector list contains .cls (exact class),
    including grouped selectors (`.a,\n.b { ... }`)."""
    pattern = re.compile(r"\.%s(?![A-Za-z0-9_-])" % re.escape(cls))
    for m in pattern.finditer(css):
        # scan forward to the opening brace of this selector group
        j = m.end()
        while j < len(css) and css[j] not in "{}":
            j += 1
        if j < len(css) and css[j] == "{":
            depth = 1
            k = j + 1
            body = []
            while k < len(css) and depth > 0:
                if css[k] == "{":
                    depth += 1
                elif css[k] == "}":
                    depth -= 1
                    if depth == 0:
                        break
                body.append(css[k])
                k += 1
            return "".join(body).strip().replace("\n", " ")
    return "<NO RULE>"


def main():
    css = read(GLOBALS)
    report = []
    for path in FILES:
        base = baseline_source(path)
        cur = read(path)
        report.append(f"## {path}")
        report.append("")
        report.append("### d5903cf inline styles (source)")
        for lineno, (tag, _), body in extract_styles(base):
            report.append(f"- L{lineno} on <{tag}>: {body}")
        report.append("")
        report.append("### current classes (with rule bodies)")
        seen = set()
        for lineno, cls in extract_classes(cur):
            key = (lineno, cls)
            if key in seen:
                continue
            seen.add(key)
            report.append(f"- L{lineno}: {cls} => {rule_body(css, cls)}")
        report.append("")
    print("\n".join(report))


if __name__ == "__main__":
    sys.exit(main())
