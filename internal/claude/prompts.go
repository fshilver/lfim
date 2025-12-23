package claude

import "fmt"

const readOnlyConstraints = `## System Constraints
You are in READ-ONLY analysis mode.

FORBIDDEN ACTIONS:
- Do NOT use Write, Edit, NotebookEdit, or any file modification tools
- Do NOT request file write permissions
- Do NOT create, modify, or save any files
- Do NOT output messages like "파일 작성을 위해 권한이 필요합니다"

REQUIRED BEHAVIOR:
- Return ALL content as your direct text response
- Start responses immediately with content (no preamble or meta-commentary)
- The host application will handle file saving automatically

`

// BuildAnalysisPrompt builds the analysis prompt
func BuildAnalysisPrompt(briefContent, briefPath string) string {
	return fmt.Sprintf(`%s## Task
Analyze this issue and provide:
1. Root cause / Feature scope
2. Implementation options with pros/cons
3. Recommended approach
4. Risk assessment

Issue (%s):
%s

## Output Format
Return markdown content directly as your response text.
Do NOT wrap output in code blocks.
Start immediately with the first section header.`, readOnlyConstraints, briefPath, briefContent)
}

// BuildPlanPrompt builds the plan prompt
func BuildPlanPrompt(briefContent, analysisContent string) string {
	return fmt.Sprintf(`%s## Task
Create a detailed implementation plan based on the issue brief and analysis.

### Brief
%s

### Analysis
%s

## Output Format
Return markdown content directly. Start with the first section header.

### Required Sections

## Plan Summary
- 3-5 bullet points summarizing the approach

## Implementation Tasks
Numbered list of specific tasks:
1. Task description
   - File: path/to/file.py
   - Changes: Description of modifications

## Files Modified
| File | Changes |
|------|---------|
| path/to/file.py | Description |

## Testing Approach
- How to verify the implementation

## Risk Mitigation
- Potential issues and how to handle them`, readOnlyConstraints, briefContent, analysisContent)
}

// BuildReviewPrompt builds the review/refinement prompt
func BuildReviewPrompt(currentAnalysis, feedback string) string {
	return fmt.Sprintf(`%s## Context
You previously provided the following analysis:

---
%s
---

## User Feedback
The user has reviewed your analysis and provided the following feedback:

%s

## Task
Please revise your analysis based on this feedback. Maintain the same structure
but address the specific concerns or suggestions raised.

## Output Format
Return the complete revised analysis as markdown.
Start immediately with the first section header.
Do NOT wrap output in code blocks.`, readOnlyConstraints, currentAnalysis, feedback)
}

// BuildPlanReviewPrompt builds the plan review/refinement prompt
func BuildPlanReviewPrompt(currentPlan, feedback string) string {
	return fmt.Sprintf(`%s## Context
You previously provided the following implementation plan:

---
%s
---

## User Feedback
The user has reviewed your plan and provided the following feedback:

%s

## Task
Please revise your implementation plan based on this feedback. Maintain the same structure
but address the specific concerns or suggestions raised.

## Output Format
Return the complete revised plan as markdown.
Start immediately with the first section header.
Do NOT wrap output in code blocks.`, readOnlyConstraints, currentPlan, feedback)
}

// BuildCommitMessagePrompt builds the commit message prompt
func BuildCommitMessagePrompt(issueID, content string) string {
	return fmt.Sprintf(`Generate a git commit message for closing issue %s.

Context:
%s

Requirements:
- First line: type(scope): brief description (max 72 chars)
- Types: feat, fix, refactor, docs, chore
- Blank line after first line
- Body: bullet points explaining key changes
- Footer: Issue: #%s

Output ONLY the commit message, no explanations.
Do NOT wrap the output in code blocks or backticks.`, issueID, content, issueID)
}
