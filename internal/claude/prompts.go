package claude

import (
	"encoding/json"
	"fmt"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
)

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
- Potential issues and how to handle them

---

## Change Log

### [Initial Plan]
- Plan created based on analysis`, readOnlyConstraints, briefContent, analysisContent)
}

// BuildReviewPrompt builds the review/refinement prompt
func BuildReviewPrompt(analysisPath, feedback string) string {
	return fmt.Sprintf(`%s## Context
The current analysis is in: %s

## User Feedback
%s

## Task
Read the analysis file and revise it based on the feedback following these CRITICAL rules:

1. **PRESERVE ALL EXISTING CONTENT** - Keep ALL sections and content that are NOT directly addressed by the feedback. Do NOT remove or summarize existing analysis.
2. **ADD rather than REPLACE** - When adding new suggestions or alternatives, APPEND them to existing content rather than replacing what was there.
3. **MAINTAIN ALL SECTION HEADERS** - Keep every original section header (Root Cause, Options, Risk Assessment, etc.). Do NOT remove any sections.
4. **ONLY MODIFY RELEVANT SECTIONS** - If feedback is about a specific topic (e.g., alternative solutions), only modify that specific section while keeping all other sections EXACTLY as they were.
5. **BE ADDITIVE** - If the user suggests a new approach, add it as an ADDITIONAL option rather than removing existing analysis.

IMPORTANT: The revised analysis MUST contain all the same section headers as the original. Missing sections is a critical error.

## Output Format
Return the complete revised analysis as markdown.
Start immediately with the first section header.
Do NOT wrap output in code blocks.`, readOnlyConstraints, analysisPath, feedback)
}

// BuildPlanReviewPrompt builds the plan review/refinement prompt
func BuildPlanReviewPrompt(planPath, feedback string) string {
	return fmt.Sprintf(`%s## Context
The current implementation plan is in: %s

## User Feedback
%s

## Task
Read the plan file and revise it based on the feedback following these CRITICAL rules:

1. **PRESERVE ALL EXISTING CONTENT** - Keep ALL sections and content that are NOT directly addressed by the feedback. Do NOT remove or summarize existing plan details.
2. **ADD rather than REPLACE** - When adding new tasks or modifications, APPEND them to existing content rather than replacing what was there.
3. **MAINTAIN ALL SECTION HEADERS** - Keep every original section header (Plan Summary, Implementation Tasks, Files Modified, Testing Approach, Risk Mitigation, etc.). Do NOT remove any sections.
4. **ONLY MODIFY RELEVANT SECTIONS** - If feedback is about a specific topic (e.g., testing approach), only modify that specific section while keeping all other sections EXACTLY as they were.
5. **BE ADDITIVE** - If the user suggests a new task or approach, add it as an ADDITIONAL item rather than removing existing plan content.

IMPORTANT: The revised plan MUST contain all the same section headers as the original. Missing sections is a critical error.

## Output Format
Return the complete revised plan as markdown.
Start immediately with the first section header.
Do NOT wrap output in code blocks.`, readOnlyConstraints, planPath, feedback)
}

// BuildImplementPrompt builds the implementation prompt for interactive mode
func BuildImplementPrompt(planPath string) string {
	return fmt.Sprintf(`IMPORTANT: The plan file may have been edited since your last read.
First, re-read %s from disk to get the latest version before implementing.

Then implement all tasks in the plan in order.
After each significant change, verify it works correctly.`, planPath)
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

const analysisJSONSchema = `{
  "summary": "Brief summary of the issue and analysis",
  "root_cause": "Root cause analysis or feature scope description",
  "options": [
    {
      "id": "opt1",
      "title": "Option title",
      "description": "Brief description of this approach",
      "pros": ["Advantage 1", "Advantage 2"],
      "cons": ["Disadvantage 1", "Disadvantage 2"],
      "recommended": true,
      "details": "Detailed explanation in markdown format..."
    }
  ],
  "risk_assessment": "Overall risk assessment and considerations"
}`

// BuildAnalysisPromptJSON builds the analysis prompt for JSON output
func BuildAnalysisPromptJSON(briefContent, briefPath string) string {
	return fmt.Sprintf(`%s## Task
Analyze this issue and provide structured analysis in JSON format.

Issue (%s):
%s

## Output Format
Return a JSON object with the following structure:
%s

## Requirements
1. Provide at least 2-3 implementation options
2. Mark exactly ONE option as "recommended": true
3. Each option must have at least 2 pros and 2 cons
4. The "details" field should contain a comprehensive explanation in markdown format
5. Do NOT wrap the JSON in code blocks - return raw JSON only
6. Ensure valid JSON syntax (proper escaping of special characters in strings)

Return ONLY the JSON object, no additional text.`, readOnlyConstraints, briefPath, briefContent, analysisJSONSchema)
}

// BuildPlanPromptWithOption builds the plan prompt with selected option context
func BuildPlanPromptWithOption(briefContent string, analysis *model.Analysis) string {
	selectedOption := analysis.GetSelectedOption()
	if selectedOption == nil {
		// Fallback to regular plan prompt if no option selected
		analysisJSON, _ := json.MarshalIndent(analysis, "", "  ")
		return BuildPlanPrompt(briefContent, string(analysisJSON))
	}

	return fmt.Sprintf(`%s## Task
Create a detailed implementation plan based on the issue brief and selected approach.

### Brief
%s

### Analysis Summary
%s

### Selected Approach
**%s**: %s

#### Details
%s

#### Pros
%s

#### Cons
%s

## Output Format
Return markdown content directly. Start with the first section header.

### Required Sections

## Plan Summary
- 3-5 bullet points summarizing the approach based on the SELECTED option

## Implementation Tasks
Numbered list of specific tasks:
1. Task description
   - File: path/to/file
   - Changes: Description of modifications

## Files Modified
| File | Changes |
|------|---------|
| path/to/file | Description |

## Testing Approach
- How to verify the implementation

## Risk Mitigation
- Potential issues and how to handle them (consider the cons of the selected approach)

---

## Change Log

### [Initial Plan]
- Plan created based on selected option: %s`,
		readOnlyConstraints,
		briefContent,
		analysis.Summary,
		selectedOption.Title,
		selectedOption.Description,
		selectedOption.Details,
		formatList(selectedOption.Pros),
		formatList(selectedOption.Cons),
		selectedOption.Title)
}

// BuildAddOptionPrompt builds the prompt for adding a custom option
func BuildAddOptionPrompt(analysis *model.Analysis, userDescription string) string {
	existingOptions := ""
	for _, opt := range analysis.Options {
		existingOptions += fmt.Sprintf("- %s: %s\n", opt.ID, opt.Title)
	}

	return fmt.Sprintf(`%s## Context
The user is reviewing an issue analysis and wants to add a custom implementation option.

### Current Analysis Summary
%s

### Existing Options
%s

### User's Description of New Option
%s

## Task
Create a new option based on the user's description. Return a JSON object for the single option:

{
  "id": "opt_custom_N",
  "title": "Short descriptive title",
  "description": "Brief one-line description",
  "pros": ["Advantage 1", "Advantage 2", "..."],
  "cons": ["Disadvantage 1", "Disadvantage 2", "..."],
  "recommended": false,
  "details": "Detailed markdown explanation of this approach..."
}

## Requirements
1. Generate a unique ID (e.g., "opt_custom_1" if no custom options exist)
2. Provide at least 2 pros and 2 cons
3. Set "recommended": false (user will choose if they want this option)
4. The "details" field should be comprehensive
5. Do NOT wrap the JSON in code blocks - return raw JSON only

Return ONLY the JSON object for this single option.`, readOnlyConstraints, analysis.Summary, existingOptions, userDescription)
}

// BuildChangeLogPrompt builds the prompt for generating a change log entry
func BuildChangeLogPrompt(planContent, gitDiff, changeReason string) string {
	return fmt.Sprintf(`%s## Context
You are analyzing the difference between an implementation plan and the actual implementation.

### Original Plan
%s

### Git Diff (actual changes made)
%s

### User's Description of Changes (optional)
%s

## Task
Generate a Change Log entry that documents the deviations from the original plan.
Focus on MEANINGFUL differences - not cosmetic changes or minor implementation details.

## Output Format
Return ONLY the Change Log entry in this exact format (no additional text):

### [%s] Implementation Update
- **Changes Made**: [List the actual changes that deviated from the plan]
- **Reason**: [Explain why the changes were necessary]
- **Affected Files**: [List files that were changed differently than planned]

Important:
- If there are no significant deviations from the plan, return: "No significant deviations from plan."
- Be concise but informative
- Focus on the "why" not just the "what"`, readOnlyConstraints, planContent, gitDiff, changeReason, "{{DATE}}")
}

// formatList formats a slice of strings as a bulleted list
func formatList(items []string) string {
	result := ""
	for _, item := range items {
		result += fmt.Sprintf("- %s\n", item)
	}
	return result
}
