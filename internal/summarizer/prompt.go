package summarizer

import (
	"fmt"
	"strings"

	"github.com/cengsin/system-agent-rag/internal/model"
)

const systemPrompt = `You are a file system analyst. Given a list of directories with their metadata, generate a concise one-line description (max 120 characters) for each directory.

Output format: one line per directory in the exact format:
PATH|DESCRIPTION

Where PATH is the exact directory path provided and DESCRIPTION is your one-line summary.
Do not include headers, explanations, or any text outside this format.`

func buildUserPrompt(dirs []model.FileInfo) string {
	var b strings.Builder
	b.WriteString("Analyze the following directories. For each, provide a one-line description.\n\n")

	for _, d := range dirs {
		b.WriteString("---\n")
		fmt.Fprintf(&b, "Path: %s\n", d.Path)
		fmt.Fprintf(&b, "Name: %s\n", d.Name)
		fmt.Fprintf(&b, "Modified: %s\n", d.ModTime.Format("2006-01-02 15:04:05"))
		b.WriteString("---\n")
	}

	return b.String()
}
