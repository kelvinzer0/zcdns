package api

import (
	"strings"
)

// thinkingStreamFilter intercepts reasoning content and raw thinking tags
// (<think>, </think>, <thinking>, </thinking>, <thought>, </thought>)
// and transforms them into OpenWebUI's native collapsible reasoning format:
// <details type="reasoning" done="true">\n<summary>Thought</summary>\n\n...\n</details>\n\n
type thinkingStreamFilter struct {
	inReasoning        bool
	fromReasoningDelta bool
	buffer             string
	lastChar           rune
	hasEmitted         bool
}

func newThinkingStreamFilter() *thinkingStreamFilter {
	return &thinkingStreamFilter{}
}

// isPotentialTagPrefix checks if s starting with '<' could be a prefix of our supported tags
func isPotentialTagPrefix(s string) bool {
	if len(s) == 0 || s[0] != '<' {
		return false
	}
	if len(s) > 20 {
		return false
	}
	lower := strings.ToLower(s)
	candidates := []string{
		"<think>", "<thinking>", "<thought>",
		"</think>", "</thinking>", "</thought>",
	}
	for _, cand := range candidates {
		if strings.HasPrefix(cand, lower) {
			return true
		}
	}
	return false
}

func isOpeningThinkTag(tag string) bool {
	lower := strings.ToLower(strings.TrimSpace(tag))
	if !strings.HasPrefix(lower, "<") || !strings.HasSuffix(lower, ">") {
		return false
	}
	inner := strings.TrimSpace(lower[1 : len(lower)-1])
	return inner == "think" || inner == "thinking" || inner == "thought"
}

func isClosingThinkTag(tag string) bool {
	lower := strings.ToLower(strings.TrimSpace(tag))
	if !strings.HasPrefix(lower, "</") || !strings.HasSuffix(lower, ">") {
		return false
	}
	inner := strings.TrimSpace(lower[2 : len(lower)-1])
	return inner == "think" || inner == "thinking" || inner == "thought"
}

func stripThinkTags(text string) string {
	res := text
	for _, tag := range []string{
		"<think>", "</think>", "<thinking>", "</thinking>", "<thought>", "</thought>",
		"<think/>", "<thinking/>", "<thought/>",
	} {
		res = strings.ReplaceAll(res, tag, "")
		res = strings.ReplaceAll(res, strings.ToUpper(tag), "")
	}
	return res
}

func (f *thinkingStreamFilter) updateLastChar(s string) {
	if len(s) > 0 {
		f.hasEmitted = true
		r := []rune(s)
		f.lastChar = r[len(r)-1]
	}
}

func (f *thinkingStreamFilter) openReasoning() string {
	f.inReasoning = true
	var sb strings.Builder
	if f.hasEmitted {
		if f.lastChar != '\n' {
			sb.WriteString("\n\n")
		} else {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("<details type=\"reasoning\" done=\"true\">\n<summary>Thought</summary>\n\n")
	res := sb.String()
	f.updateLastChar(res)
	return res
}

func (f *thinkingStreamFilter) closeReasoning() string {
	f.inReasoning = false
	f.fromReasoningDelta = false
	res := "\n</details>\n\n"
	f.updateLastChar(res)
	return res
}

// ProcessReasoning handles explicit reasoning_content chunks (e.g. from DeepSeek R1, o1, or Anthropic thinking)
func (f *thinkingStreamFilter) ProcessReasoning(reasoning string) string {
	clean := stripThinkTags(reasoning)
	if clean == "" {
		return ""
	}

	var sb strings.Builder
	if !f.inReasoning {
		sb.WriteString(f.openReasoning())
	}
	f.fromReasoningDelta = true

	sb.WriteString(clean)
	f.updateLastChar(clean)
	return sb.String()
}

// ProcessContent handles standard content stream chunks, intercepting <think>...</think> tags
func (f *thinkingStreamFilter) ProcessContent(content string) string {
	if content == "" {
		return ""
	}

	var sb strings.Builder

	// If we were in a reasoning block started by ProcessReasoning, non-empty content marks the end of reasoning
	// unless the content itself starts with a closing tag which will be handled below.
	if f.inReasoning && f.fromReasoningDelta {
		trimmed := strings.TrimSpace(content)
		if !strings.HasPrefix(trimmed, "</") {
			sb.WriteString(f.closeReasoning())
		}
	}

	input := f.buffer + content
	f.buffer = ""

	for len(input) > 0 {
		ltIdx := strings.IndexByte(input, '<')
		if ltIdx == -1 {
			// No '<' in input
			sb.WriteString(input)
			f.updateLastChar(input)
			break
		}

		// Text before '<'
		if ltIdx > 0 {
			before := input[:ltIdx]
			sb.WriteString(before)
			f.updateLastChar(before)
			input = input[ltIdx:]
		}

		// Now input starts with '<'
		gtIdx := strings.IndexByte(input, '>')
		if gtIdx == -1 {
			// Incomplete tag at tail of chunk
			if isPotentialTagPrefix(input) {
				f.buffer = input
				break
			}
			// Not a potential think tag prefix, emit first char and continue
			charToEmit := input[:1]
			sb.WriteString(charToEmit)
			f.updateLastChar(charToEmit)
			input = input[1:]
			continue
		}

		// We have a full tag: input[:gtIdx+1]
		tag := input[:gtIdx+1]
		rest := input[gtIdx+1:]

		if isOpeningThinkTag(tag) {
			if !f.inReasoning {
				sb.WriteString(f.openReasoning())
			}
			input = rest
			// Consume optional immediate single newline after opening tag
			if strings.HasPrefix(input, "\r\n") {
				input = input[2:]
			} else if strings.HasPrefix(input, "\n") {
				input = input[1:]
			}
			continue
		}

		if isClosingThinkTag(tag) {
			if f.inReasoning {
				sb.WriteString(f.closeReasoning())
			}
			input = rest
			// Consume optional immediate single newline after closing tag
			if strings.HasPrefix(input, "\r\n") {
				input = input[2:]
			} else if strings.HasPrefix(input, "\n") {
				input = input[1:]
			}
			continue
		}

		// Any other HTML or XML tag (e.g. <b>, <span>, <pre>, etc.)
		sb.WriteString(tag)
		f.updateLastChar(tag)
		input = rest
	}

	return sb.String()
}

// Flush closes any open reasoning block and emits any buffered characters
func (f *thinkingStreamFilter) Flush() string {
	var sb strings.Builder
	if f.buffer != "" {
		sb.WriteString(f.buffer)
		f.updateLastChar(f.buffer)
		f.buffer = ""
	}
	if f.inReasoning {
		sb.WriteString(f.closeReasoning())
	}
	return sb.String()
}

// IsInReasoning returns whether filter is currently inside a reasoning block
func (f *thinkingStreamFilter) IsInReasoning() bool {
	return f.inReasoning
}
