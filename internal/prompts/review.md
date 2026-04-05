You are reviewing a commit in the {{.RepoName}} repository.

## Commit
- SHA: {{.SHA}}
- Author: {{.Author}}
- Date: {{.Date}}
- Message: {{.Message}}

## Full Diff
```diff
{{.Diff}}
```
{{if .Annotations}}

## Reviewer's Annotations
The reviewer has commented on specific lines of the diff above (1-indexed).
{{range .Annotations}}

### Lines {{.LineStart}}–{{.LineEnd}}
```
{{.Context}}
```
> {{.Comment}}
{{end}}
{{end}}

## Instructions
{{if .Annotations -}}
Address each annotation with specific, actionable feedback. For each:
1. Is the concern valid?
2. If there's an issue, suggest a concrete fix
3. Reference specific lines from the diff

Also flag any additional issues in the diff not covered by the annotations.
{{- else -}}
Review this commit for:
1. Bugs, logic errors, or edge cases
2. Security concerns
3. Performance issues
4. Code quality, readability, and naming
5. Missing error handling

Reference specific lines from the diff when noting issues.
{{- end}}
Keep your response concise and technical. Use markdown formatting.
