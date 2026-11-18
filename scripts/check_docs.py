"""Offline checks for local Markdown links, headings, fences, and whitespace.

Checks root Markdown documents, docs/, and examples/, never .ceiling. Code spans/fences
are excluded from link checks. This is a deliberately small repository checker,
not a full Markdown renderer or an HTTP link crawler.
"""
import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit


def prose(text):
    lines = []
    fence = None
    for line in text.splitlines():
        marker = re.match(r"^\s*(`{3,}|~{3,})", line)
        if marker:
            token = marker.group(1)
            if fence is None:
                fence = token
            elif token[0] == fence[0] and len(token) >= len(fence):
                fence = None
            continue
        if fence is None:
            lines.append(line)
    return "\n".join(lines), fence is not None


def heading_ids(text):
    result = set()
    counts = {}
    content, _ = prose(text)
    for title in re.findall(r"^#{1,6}\s+(.+?)(?:\s+#+)?$", content, re.M):
        slug = re.sub(r"[^\w\- ]", "", title.lower()).replace(" ", "-")
        count = counts.get(slug, 0)
        result.add(slug if count == 0 else f"{slug}-{count}")
        counts[slug] = count + 1
    return result


def check_documents(root):
    root = Path(root).resolve()
    documents = sorted(set(root.glob("*.md")) | set((root / "docs").rglob("*.md")) | set((root / "examples").rglob("*.md")))
    documents = [p for p in documents if ".ceiling" not in p.relative_to(root).parts]
    errors = []
    for document in documents:
        label = document.relative_to(root)
        text = document.read_text(encoding="utf-8")
        for number, line in enumerate(text.splitlines(), 1):
            if line.rstrip() != line:
                errors.append(f"{label}:{number}: trailing whitespace")
        content, unclosed = prose(text)
        if unclosed:
            errors.append(f"{label}: unclosed fence")
        content = re.sub(r"(`+).*?\1", "", content)
        destinations = re.findall(r"\]\(\s*(<[^>]+>|[^\s)]+)(?:\s+\"[^\"]*\")?\s*\)", content)
        destinations += re.findall(r"^\s*\[[^\]]+\]:\s*(<[^>]+>|\S+)", content, re.M)
        for destination in destinations:
            destination = destination.strip("<>")
            url = urlsplit(destination)
            if url.scheme or url.netloc:
                continue
            target = unquote(url.path)
            if target.startswith("/"):
                path = root / target.lstrip("/")
            else:
                path = document.parent / target if target else document
            path = path.resolve()
            if not path.is_relative_to(root):
                errors.append(f"{label}: link outside repository: {destination}")
            elif not path.exists():
                errors.append(f"{label}: missing target: {destination}")
            elif url.fragment and path.suffix == ".md":
                anchors = heading_ids(path.read_text(encoding="utf-8"))
                if unquote(url.fragment) not in anchors:
                    errors.append(f"{label}: missing anchor: {destination}")
    return errors


if __name__ == "__main__":
    failures = check_documents(Path(__file__).resolve().parents[1])
    if failures:
        print("\n".join(failures), file=sys.stderr)
        raise SystemExit(1)
    print("Documentation checks passed (local links, anchors, fences, whitespace).")
