package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1"

// openRouterRequest is the request body sent to OpenRouter
type openRouterRequest struct {
	Model    string        `json:"model"`
	Messages []messageRole `json:"messages"`
	Provider *providerOpts `json:"provider,omitempty"`
}

type messageRole struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type providerOpts struct {
	Only []string `json:"only,omitempty"`
}

// openRouterResponse is the response from OpenRouter chat completions API
type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"message"`
	} `json:"choices"`
}

// generateRequest is the request body for our generate endpoint
type generateRequest struct {
	Prompt string `json:"prompt"`
}

// generateResponse is the response from our generate endpoint
type generateResponse struct {
	Response string `json:"response"`
}

func (app *application) generateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"}, nil)
		return
	}

	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"}, nil)
		return
	}

	if req.Prompt == "" {
		app.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"}, nil)
		return
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		app.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "OPENROUTER_API_KEY not configured"}, nil)
		return
	}

	openRouterReq := openRouterRequest{
		Model: "openai/gpt-oss-20b:nitro",
		Messages: []messageRole{
			{Role: "user", Content: req.Prompt},
		},
	}

	body, err := json.Marshal(openRouterReq)
	if err != nil {
		app.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to build request"}, nil)
		return
	}

	httpReq, err := http.NewRequest(http.MethodPost, openRouterBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		app.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create request"}, nil)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		app.logger.Error("OpenRouter request failed", "error", err)
		app.writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Failed to reach OpenRouter"}, nil)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.logger.Error("OpenRouter returned error", "status", resp.StatusCode)
		app.writeJSON(w, http.StatusBadGateway, map[string]string{"error": "OpenRouter request failed"}, nil)
		return
	}

	var openRouterResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		app.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse OpenRouter response"}, nil)
		return
	}

	response := ""
	if len(openRouterResp.Choices) > 0 {
		response = openRouterResp.Choices[0].Message.Content
	}

	app.writeJSON(w, http.StatusOK, generateResponse{Response: response}, nil)
}
