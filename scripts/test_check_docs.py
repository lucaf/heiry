import tempfile
import unittest
from pathlib import Path

from check_docs import check_documents


class DocumentChecks(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "docs").mkdir()

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        return path

    def test_valid_links_and_code_are_distinguished(self):
        self.write("README.md", "[Design](docs/design.md#scope-and-state)\n`Bind[T](provider)`\n```go\nBind[T](provider)\n```\n")
        self.write("docs/design.md", "# Scope and state\n")
        self.assertEqual(check_documents(self.root), [])

    def test_missing_target_and_anchor_fail(self):
        self.write("README.md", "[Missing](docs/missing.md)\n[Anchor](docs/design.md#absent)\n")
        self.write("docs/design.md", "# Present\n")
        errors = check_documents(self.root)
        self.assertTrue(any("missing target" in e for e in errors))
        self.assertTrue(any("missing anchor" in e for e in errors))

    def test_unclosed_fence_and_whitespace_fail(self):
        self.write("README.md", "# Title \n```go\n")
        errors = check_documents(self.root)
        self.assertTrue(any("unclosed fence" in e for e in errors))
        self.assertTrue(any("trailing whitespace" in e for e in errors))

    def test_relative_reference_style_links_and_duplicate_headings(self):
        self.write("docs/design.md", "# Name\n# Name\n[one][target]\n[target]: ../README.md#start\n")
        self.write("README.md", "# Start\n[second](docs/design.md#name-1)\n")
        self.assertEqual(check_documents(self.root), [])
        self.write("docs/design.md", "[bad][target]\n[target]: missing.md\n")
        self.assertTrue(any("missing target" in e for e in check_documents(self.root)))

    def test_external_urls_and_ceiling_are_excluded(self):
        self.write("README.md", "[Go](https://go.dev/)\n")
        self.write(".ceiling/notes.md", "[broken](absent.md)\n")
        self.assertEqual(check_documents(self.root), [])

    def test_path_cannot_escape_repository(self):
        self.write("README.md", "[outside](../outside.md)\n")
        self.assertTrue(any("outside repository" in e for e in check_documents(self.root)))

    def test_examples_documents_are_checked(self):
        self.write("examples/README.md", "[program](basic/main.go)\n[guide](../docs/guide.md#usage)\n")
        self.write("examples/basic/main.go", "package main\n")
        self.write("docs/guide.md", "# Usage\n")
        self.assertEqual(check_documents(self.root), [])
        self.write("examples/README.md", "[missing](absent/main.go)\n")
        self.assertTrue(any("examples/README.md: missing target" in e for e in check_documents(self.root)))


if __name__ == "__main__":
    unittest.main()
