package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Modelos em ordem de preferência (fallback)
// Nota: o "gemini-flash-lite-latest" é o que funciona com a chave actual
var modelosFallback = []string{
	"gemini-flash-lite-latest",
	"gemini-flash-latest",
	"gemini-2.5-flash-lite",
	"gemini-2.5-flash",
}

// ChamarGemini tenta vários modelos até um funcionar
func ChamarGemini(ctx context.Context, apiKey string, prompt string) (string, error) {
	client := &http.Client{Timeout: 60 * time.Second}

	var ultimoErro error

	for _, modelo := range modelosFallback {
		url := fmt.Sprintf(
			"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
			modelo,
		)

		payload := map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": prompt}}},
			},
			"generationConfig": map[string]interface{}{
				"temperature":     0.7,
				"maxOutputTokens": 2048,
				"topP":            0.95,
			},
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			ultimoErro = err
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			ultimoErro = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			ultimoErro = fmt.Errorf("%s: %v", modelo, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("⚠️  %s: HTTP %d\n", modelo, resp.StatusCode)
			ultimoErro = fmt.Errorf("%s: HTTP %d", modelo, resp.StatusCode)
			continue
		}

		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			ultimoErro = fmt.Errorf("%s: %v", modelo, err)
			continue
		}

		if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
			ultimoErro = fmt.Errorf("%s: resposta vazia", modelo)
			continue
		}

		// Sucesso!
		fmt.Printf("✅ %s: resposta gerada\n", modelo)
		return result.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("todos os modelos falharam: %v", ultimoErro)
}
