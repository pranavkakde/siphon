package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
)

const (
	maxArtifactBytes = 4096 // 4 KB per artifact field to stay within token budgets
)

// AIAnalysis is the structured result written back to MongoDB.
type AIAnalysis struct {
	Category     string    `bson:"category"      json:"category"`
	Confidence   float64   `bson:"confidence"    json:"confidence"`
	RootCause    string    `bson:"root_cause"    json:"root_cause"`
	SuggestedFix string    `bson:"suggested_fix" json:"suggested_fix"`
	AnalyzedAt   time.Time `bson:"analyzed_at"   json:"analyzed_at"`
	Model        string    `bson:"model"         json:"model"`
	Provider     string    `bson:"provider"      json:"provider"`
}

// ProviderConfig holds LLM connection details loaded from the siphon_settings collection.
type ProviderConfig struct {
	Provider string // "openai" | "anthropic" | "openai_compatible"
	APIKey   string
	Model    string
	BaseURL  string // used for openai_compatible (e.g. Ollama at http://localhost:11434)
}

// Analyzer builds the SDET prompt and calls the configured LLM provider.
type Analyzer struct {
	promptTmpl *template.Template
}

// New creates a new Analyzer instance.
func New() (*Analyzer, error) {
	tmpl, err := template.New("sdet").Parse(SDETPromptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prompt template: %w", err)
	}
	return &Analyzer{promptTmpl: tmpl}, nil
}

// Analyze runs the full LLM pipeline for a single failed test case.
func (a *Analyzer) Analyze(ctx context.Context, cfg ProviderConfig, data PromptData) (*AIAnalysis, error) {
	// Truncate oversized artifact fields to stay within token budgets
	data.DOMSnapshot = truncateUTF8(data.DOMSnapshot, maxArtifactBytes)
	data.HARData = truncateUTF8(data.HARData, maxArtifactBytes)
	data.ErrorTrace = truncateUTF8(data.ErrorTrace, maxArtifactBytes)

	// Build the prompt
	var buf bytes.Buffer
	if err := a.promptTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("prompt template execution failed: %w", err)
	}
	prompt := buf.String()

	// Dispatch to the correct provider
	var rawJSON string
	var err error
	switch strings.ToLower(cfg.Provider) {
	case "anthropic":
		rawJSON, err = callAnthropic(ctx, cfg, prompt)
	case "openai_compatible":
		rawJSON, err = callOpenAICompatible(ctx, cfg, prompt)
	default: // "openai" (default)
		rawJSON, err = callOpenAI(ctx, cfg, prompt)
	}
	if err != nil {
		return nil, err
	}

	// Parse the LLM JSON response
	var result AIAnalysis
	if jsonErr := json.Unmarshal([]byte(rawJSON), &result); jsonErr != nil {
		return nil, fmt.Errorf("LLM returned non-parseable JSON: %w — raw: %s", jsonErr, rawJSON)
	}

	result.AnalyzedAt = time.Now()
	result.Model = cfg.Model
	result.Provider = cfg.Provider

	return &result, nil
}

// ─── Provider Implementations ───────────────────────────────────────────────

func callOpenAI(ctx context.Context, cfg ProviderConfig, prompt string) (string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature":     0.1,
		"response_format": map[string]string{"type": "json_object"},
	}
	return doOpenAIStyleRequest(ctx, baseURL+"/v1/chat/completions", cfg.APIKey, "Bearer", body)
}

func callOpenAICompatible(ctx context.Context, cfg ProviderConfig, prompt string) (string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434" // Ollama default
	}
	model := cfg.Model
	if model == "" {
		model = "llama3"
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"stream":      false,
	}
	return doOpenAIStyleRequest(ctx, baseURL+"/v1/chat/completions", cfg.APIKey, "Bearer", body)
}

func callAnthropic(ctx context.Context, cfg ProviderConfig, prompt string) (string, error) {
	model := cfg.Model
	if model == "" {
		model = "claude-3-5-haiku-20241022"
	}

	reqBody := map[string]any{
		"model":      model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic returned %d: %s", resp.StatusCode, string(respBytes))
	}

	// Anthropic response: content[0].text
	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &anthropicResp); err != nil || len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("failed to parse anthropic response: %s", string(respBytes))
	}
	return extractJSON(anthropicResp.Content[0].Text), nil
}

// doOpenAIStyleRequest handles both OpenAI and OpenAI-compatible (Ollama) endpoints.
func doOpenAIStyleRequest(ctx context.Context, url, apiKey, scheme string, body map[string]any) (string, error) {
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", scheme+" "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	// OpenAI-style response: choices[0].message.content
	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &openaiResp); err != nil || len(openaiResp.Choices) == 0 {
		return "", fmt.Errorf("failed to parse LLM response: %s", string(respBytes))
	}
	return extractJSON(openaiResp.Choices[0].Message.Content), nil
}

// extractJSON finds the first JSON object in a potentially freeform LLM response.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}

// truncateUTF8 safely trims a string to at most maxBytes bytes, respecting UTF-8 boundaries.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	truncated := s[:maxBytes]
	// Walk back to a valid rune boundary
	for !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "\n[... truncated ...]"
}
