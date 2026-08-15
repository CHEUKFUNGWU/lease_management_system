#!/usr/bin/env python3
"""STY-009: convert static inline styles to generated classes.

For each target file, every `style={{...}}` whose body is fully static
(no template refs, no identifiers beyond var() calls, no conditionals)
becomes `className="sty-<hash8>"`; the rule is appended to globals.css in
a STY-009 section. Dynamic styles (conditionals, variable references)
stay inline — the design guard only governs static ones.

Hash is a stable short digest of the *normalized* body (sorted keys) so
identical styles share one class. camelCase keys become kebab-case.
"""
import hashlib
import re
import sys

FILES = [
    "app/reports/page.tsx",
    "app/monthly-closing/page.tsx",
    "app/contracts/[id]/workspace/ContractWorkspaceDialogs.tsx",
    "app/contracts/page.tsx",
    "app/reports/components/BudgetVariancePanel.tsx",
]
CSS_PATH = "app/globals.css"


def extract_styles(src):
    """Yield (start, end, body) for every style={{...}} block."""
    out = []
    for m in re.finditer(r"style=\{\{", src):
        i = m.end() - 1
        depth = 1
        j = i + 1
        while j < len(src) and depth > 0:
            if src[j] == "{":
                depth += 1
            elif src[j] == "}":
                depth -= 1
            j += 1
        out.append((m.start(), j, src[i + 1 : j - 1]))
    return out


def is_static(body):
    t = re.sub(r'"[^"]*"', '""', body)
    t = re.sub(r"var\([^)]*\)", "V", t)
    t = re.sub(r"[a-zA-Z-]+:", "", t)
    t = re.sub(r"-?\d*\.?\d+(px|%|em|rem|vh|vw|ms|s|fr|deg)?", "", t)
    t = re.sub("[\\s,{}'()\"]", "", t)
    return t == "" or t == "V"


def camel_to_kebab(name):
    return re.sub(r"(?<!^)(?=[A-Z])", "-", name).lower()


def parse_body(body):
    """Parse `key: value, key: value` into list of (kebab, value)."""
    props = []
    # split top-level commas (no nested braces here since static)
    for part in body.split(","):
        part = part.strip()
        if not part:
            continue
        if ":" not in part:
            continue
        k, v = part.split(":", 1)
        props.append((camel_to_kebab(k.strip()), v.strip()))
    return props


def normalize(props):
    return sorted(props)


def rule_for(props):
    lines = [f".sty-{hash_body(props)} {{"]
    for k, v in props:
        lines.append(f"  {k}: {v};")
    lines.append("}")
    return "\n".join(lines)


def hash_body(props):
    raw = ",".join(f"{k}:{v}" for k, v in normalize(props))
    return hashlib.sha256(raw.encode()).hexdigest()[:8]


def main():
    css = open(CSS_PATH).read()
    css_entries = []  # (class, rule)
    total_static = 0
    total_dynamic = 0
    for path in FILES:
        src = open(path).read()
        blocks = extract_styles(src)
        # process from the end so offsets stay valid
        replacements = []
        for start, end, body in reversed(blocks):
            if not is_static(body):
                total_dynamic += 1
                continue
            props = parse_body(body)
            if not props:
                continue
            cls = f"sty-{hash_body(props)}"
            total_static += 1
            replacements.append((start, end, f'className="{cls}"'))
            rule = rule_for(props)
            if cls not in {c for c, _ in css_entries}:
                css_entries.append((cls, rule))
        # Step 1: merge the new class into elements that already carry a
        # className, BEFORE any style replacement so offsets are untouched.
        # The className edit is in-place (same length region rewrite is not
        # required; we do it on the original offsets while the source still
        # matches them).
        for start, end, repl in replacements:
            cls = repl.split('"')[1]
            tag_start = src.rfind("<", 0, start)
            tag_end = src.find(">", end)
            tag = src[tag_start:tag_end]
            m = re.search(r'className="([^"]*)"', tag)
            if m:
                merged = m.group(1) + " " + cls
                tag2 = tag[: m.start(1)] + merged + tag[m.end(1) :]
                src = src[:tag_start] + tag2 + src[tag_end:]
        # Step 2: replace style attributes with the generated class names,
        # from the end so earlier offsets stay valid against the original
        # (className edits above changed lengths before each style, so
        # recompute offsets by scanning for the marker).
        for start, end, repl in sorted(replacements, reverse=True):
            # find the style={{...}} nearest to the original start in the
            # current source: the className merge shifted everything after it
            idx = src.rfind("style={{", 0, start + 1)
            if idx == -1:
                continue
            depth = 1
            j = idx + len("style={{") - 1
            while j < len(src) and depth > 0:
                if src[j] == "{":
                    depth += 1
                elif src[j] == "}":
                    depth -= 1
                j += 1
            src = src[:idx] + repl + src[j:]
        open(path, "w").write(src)
    # append CSS
    header = "\n/* STY-009: static inline styles converted to classes (auto-generated). */\n"
    existing = set(re.findall(r"\.(sty-[0-9a-f]{8})", css))
    new_rules = []
    for cls, rule in css_entries:
        if cls in existing:
            continue
        new_rules.append(rule)
    if new_rules:
        css += header + "\n" + "\n\n".join(new_rules) + "\n"
        open(CSS_PATH, "w").write(css)
    print(f"static converted: {total_static}")
    print(f"dynamic kept:     {total_dynamic}")
    print(f"unique classes:   {len(css_entries)} (new rules: {len(new_rules)})")


if __name__ == "__main__":
    sys.exit(main())
