package analyzer

// SDETPromptTemplate is the expert SDET prompt sent to the LLM for every failed test case.
// It accepts a PromptData struct and uses Go text/template syntax.
const SDETPromptTemplate = `You are an expert SDET debugging a test failure. Analyze the following DOM snapshot, Network HAR, and error trace. Categorize the failure into one of the following: [Locator_Changed, API_Failure, Data_Stale, Environment_Issue]. Provide a 2-sentence root cause and a suggested code fix. Return ONLY a valid JSON object.

Test Case: {{.TestCaseName}}
Error Message: {{.ErrorMessage}}

--- DOM Snapshot (truncated to 4KB) ---
{{.DOMSnapshot}}

--- Network HAR (failed/non-2xx requests only, truncated to 4KB) ---
{{.HARData}}

--- Error Trace ---
{{.ErrorTrace}}

Respond with exactly this JSON schema and nothing else:
{
  "category": "<one of: Locator_Changed | API_Failure | Data_Stale | Environment_Issue>",
  "confidence": <float between 0.0 and 1.0>,
  "root_cause": "<2-sentence explanation of why this test failed>",
  "suggested_fix": "<concrete code change, selector update, config fix, or retry strategy>"
}`

// PromptData holds the template variables used to populate SDETPromptTemplate.
type PromptData struct {
	TestCaseName string
	ErrorMessage string
	DOMSnapshot  string
	HARData      string
	ErrorTrace   string
}
