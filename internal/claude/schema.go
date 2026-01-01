package claude

// AnalysisSchema defines the JSON Schema for structured analysis output
// This schema is used with claude-code's --json-schema option to ensure
// consistent, valid JSON responses without manual parsing and cleanup.
const AnalysisSchema = `{
  "type": "object",
  "properties": {
    "summary": {
      "type": "string",
      "description": "Brief summary of the issue and analysis"
    },
    "root_cause": {
      "type": "string",
      "description": "Root cause analysis or feature scope description"
    },
    "options": {
      "type": "array",
      "description": "Implementation options with pros/cons",
      "items": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "description": "Unique identifier for this option (e.g., opt1, opt2)"
          },
          "title": {
            "type": "string",
            "description": "Short descriptive title for this approach"
          },
          "description": {
            "type": "string",
            "description": "Brief one-line description of this approach"
          },
          "pros": {
            "type": "array",
            "description": "Advantages of this approach",
            "items": {
              "type": "string"
            }
          },
          "cons": {
            "type": "array",
            "description": "Disadvantages of this approach",
            "items": {
              "type": "string"
            }
          },
          "recommended": {
            "type": "boolean",
            "description": "Whether this is the recommended option"
          },
          "details": {
            "type": "string",
            "description": "Detailed explanation in markdown format"
          }
        },
        "required": ["id", "title", "description", "pros", "cons", "recommended", "details"]
      }
    },
    "risk_assessment": {
      "type": "string",
      "description": "Overall risk assessment and considerations"
    }
  },
  "required": ["summary", "root_cause", "options", "risk_assessment"]
}`

// OptionSchema defines the JSON Schema for a single analysis option
// Used when adding custom options to an existing analysis
const OptionSchema = `{
  "type": "object",
  "properties": {
    "id": {
      "type": "string",
      "description": "Unique identifier for this option"
    },
    "title": {
      "type": "string",
      "description": "Short descriptive title"
    },
    "description": {
      "type": "string",
      "description": "Brief one-line description"
    },
    "pros": {
      "type": "array",
      "description": "Advantages of this approach",
      "items": {
        "type": "string"
      }
    },
    "cons": {
      "type": "array",
      "description": "Disadvantages of this approach",
      "items": {
        "type": "string"
      }
    },
    "recommended": {
      "type": "boolean",
      "description": "Whether this is the recommended option"
    },
    "details": {
      "type": "string",
      "description": "Detailed explanation in markdown format"
    }
  },
  "required": ["id", "title", "description", "pros", "cons", "recommended", "details"]
}`
