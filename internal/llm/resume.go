package llm

import "strings"

const maxResumePartialChars = 12000

// ApplyResumeHint appends retry instructions so a timed-out analysis can continue
// in the same conversation (session) or start over.
func ApplyResumeHint(prompt, partial string, fresh bool) string {
	prompt = strings.TrimSpace(prompt)
	if fresh {
		if prompt == "" {
			prompt = "Please answer again."
		}
		return prompt + "\n\n[System] Previous model call timed out. Regenerate a complete answer from scratch. Ignore any incomplete draft."
	}
	partial = strings.TrimSpace(partial)
	if partial == "" {
		if prompt == "" {
			return prompt
		}
		return prompt + "\n\n[System] Previous model call timed out with no usable output. Retry the same request and return a complete answer."
	}
	if len(partial) > maxResumePartialChars {
		partial = partial[:maxResumePartialChars] + "\n…(truncated draft)…"
	}
	return prompt + "\n\n[System] Previous model call timed out. Using the incomplete draft below as context, output a COMPLETE answer from the beginning (not a delta).\n-----\n" + partial + "\n-----"
}
