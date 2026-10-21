package interviewer

import "encoding/json"

// ToolDefinition conforms to the standard OpenAI / Ollama function-calling schema.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

type FunctionSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

var AgieToolDefinitions = []ToolDefinition{
	{
		Type: "function",
		Function: FunctionSpec{
			Name: "inspect_db_catalog",
			Description: "Lists all available table names and column definitions in the manufacturing execution database. " +
				"Use this tool when you need to know the schema before generating a query.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "Optional specific table to inspect. If empty, lists all user tables.",
					},
				},
			},
		},
	},
	{
		Type: "function",
		Function: FunctionSpec{
			Name: "run_safe_sql",
			Description: "Executes a bounded, read-only SELECT query against the live factory database. " +
				"Strictly disallows mutations (INSERT, UPDATE, DELETE, DROP, ALTER). Returns rows as JSON.",
			Parameters: map[string]interface{}{
				"type": "object",
				"required": []string{"query"},
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The SELECT query to execute against PostgreSQL (e.g., 'SELECT worker_id, status FROM attendance_register WHERE status = ''ABSENT''').",
					},
				},
			},
		},
	},
}

// ToolCall represents a function call request emitted by the LLM.
type ToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}