package prompts

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed *.md
var promptFS embed.FS

var templates *template.Template

func init() {
	templates = template.Must(template.ParseFS(promptFS, "*.md"))
}

// ReviewAnnotation represents a reviewer's comment on specific diff lines.
type ReviewAnnotation struct {
	LineStart int
	LineEnd   int
	Context   string // the actual diff lines in the range
	Comment   string
}

// ReviewData is the template data for review.md.
type ReviewData struct {
	RepoName    string
	SHA         string
	Author      string
	Date        string
	Message     string
	Diff        string
	Annotations []ReviewAnnotation
	CommitNote  string // general reviewer note (not tied to specific lines)
}

// Review renders the review prompt template.
func Review(data ReviewData) (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "review.md", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
