import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("document_search", Path(__file__).with_name("document-search.py"))
acceptance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acceptance)


class TextCoverageTests(unittest.TestCase):
    def check(self, text, chunks):
        acceptance.assert_text_coverage(text, [{"text": chunk} for chunk in chunks])

    def test_repeated_text_can_have_multiple_valid_overlap_positions(self):
        self.check("abcabcabcTAIL", ["abcabc", "abcabc", "abcTAIL"])
        self.check("abcdefghi", ["abcdef", "cde", "defghi"])

    def test_gaps_reordering_tail_loss_and_escaping_are_refused(self):
        for text, chunks in [("abcdef", ["abc", "ef"]), ("abcdef", ["def", "abc"]),
                             ("abcdef", ["abcde"]), ('a "quote"', ['a &quot;quote&quot;'])]:
            with self.assertRaises(AssertionError):
                self.check(text, chunks)

    def test_surrogate_damage_and_utf16_limit_are_refused(self):
        self.check("a😀b", ["a😀", "😀b"])
        with self.assertRaises(UnicodeEncodeError):
            self.check("a😀b", ["a\ud83d", "😀b"])
        with self.assertRaises(AssertionError):
            self.check("😀" * 241, ["😀" * 241])


if __name__ == "__main__":
    unittest.main()
