package api

import (
	"strings"
	"testing"
)

func TestThinkingFilter_UserScenario(t *testing.T) {
	// Turn 1: model generates think tag before tool call
	f1 := newThinkingStreamFilter()
	chunk1 := "<think>User asks current time — requires get_current_timestamp tool. Respond only with tool call tag.</think>"
	out1 := f1.ProcessContent(chunk1) + f1.Flush()

	expectedDetailsStart := "<details type=\"reasoning\" done=\"true\">\n<summary>Thought</summary>\n\n"
	if !strings.Contains(out1, expectedDetailsStart) {
		t.Fatalf("Turn 1 expected details tag, got: %q", out1)
	}
	if strings.Contains(out1, "<think>") || strings.Contains(out1, "</think>") {
		t.Fatalf("Turn 1 leaked raw think tags: %q", out1)
	}
	if !strings.Contains(out1, "</details>") {
		t.Fatalf("Turn 1 expected closing details tag, got: %q", out1)
	}

	// Turn 2: follow up turn with garbage prefix and think tag
	f2 := newThinkingStreamFilter()
	chunk2 := "13555148<think>Just give the time.</think>Saat ini Jumat, 25 September 2026, pukul 08:26 WIB (08:26:34 pagi waktu Jakarta)."
	out2 := f2.ProcessContent(chunk2) + f2.Flush()

	if strings.Contains(out2, "<think>") || strings.Contains(out2, "</think>") {
		t.Fatalf("Turn 2 leaked raw think tags: %q", out2)
	}
	if !strings.Contains(out2, "13555148") {
		t.Fatalf("Turn 2 missing prefix text: %q", out2)
	}
	if !strings.Contains(out2, expectedDetailsStart) {
		t.Fatalf("Turn 2 expected details tag, got: %q", out2)
	}
	if !strings.Contains(out2, "Just give the time.") {
		t.Fatalf("Turn 2 missing thought body, got: %q", out2)
	}
	if !strings.Contains(out2, "Saat ini Jumat, 25 September 2026") {
		t.Fatalf("Turn 2 missing final content, got: %q", out2)
	}
}

func TestThinkingFilter_ChunkSplitting(t *testing.T) {
	f := newThinkingStreamFilter()
	chunks := []string{
		"Hello world. ",
		"<th",
		"ink>",
		"Thinking deeply...",
		"</th",
		"ink>",
		" Here is the answer.",
	}

	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString(f.ProcessContent(c))
	}
	sb.WriteString(f.Flush())

	res := sb.String()
	if strings.Contains(res, "<think>") || strings.Contains(res, "</think>") {
		t.Fatalf("Leaked raw think tags: %q", res)
	}
	if !strings.Contains(res, "<details type=\"reasoning\" done=\"true\">") {
		t.Fatalf("Expected details block: %q", res)
	}
	if !strings.Contains(res, "Thinking deeply...") {
		t.Fatalf("Expected thought body: %q", res)
	}
	if !strings.Contains(res, "Here is the answer.") {
		t.Fatalf("Expected answer text: %q", res)
	}
}

func TestThinkingFilter_ReasoningContent(t *testing.T) {
	f := newThinkingStreamFilter()
	out1 := f.ProcessReasoning("Let me think about this.")
	out2 := f.ProcessReasoning(" Still thinking.")
	out3 := f.ProcessContent("Finally, the answer.")
	outFlush := f.Flush()

	res := out1 + out2 + out3 + outFlush
	if !strings.Contains(res, "<details type=\"reasoning\" done=\"true\">\n<summary>Thought</summary>\n\nLet me think about this. Still thinking.\n</details>\n\n") {
		t.Fatalf("Unexpected reasoning format: %q", res)
	}
	if !strings.Contains(res, "Finally, the answer.") {
		t.Fatalf("Missing final answer: %q", res)
	}
}

func TestThinkingFilter_UnclosedTagFlushed(t *testing.T) {
	f := newThinkingStreamFilter()
	out1 := f.ProcessContent("<think>Unfinished thoughts")
	outFlush := f.Flush()

	res := out1 + outFlush
	if !strings.HasSuffix(strings.TrimSpace(res), "</details>") {
		t.Fatalf("Expected unclosed tag to be closed on flush, got: %q", res)
	}
}

func TestThinkingFilter_OtherHTMLTagsPreserved(t *testing.T) {
	f := newThinkingStreamFilter()
	input := "Check if 3 < 5 and use <b>bold</b> text."
	res := f.ProcessContent(input) + f.Flush()

	if res != input {
		t.Fatalf("Expected %q, got %q", input, res)
	}
}
