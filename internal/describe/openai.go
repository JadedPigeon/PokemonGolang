package describe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type OpenAI struct {
	Model   string
	APIKey  string
	Timeout time.Duration
}

func NewOpenAI(model string) *OpenAI {
	return &OpenAI{
		Model:   model,
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Timeout: 6 * time.Second,
	}
}

type chatReq struct {
	Model       string      `json:"model"`
	Temperature float32     `json:"temperature"`
	Messages    []chatMsg   `json:"messages"`
	ResponseFmt *respFormat `json:"response_format,omitempty"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
}

type aiJSON struct {
	Description string `json:"description"`
}

func (o *OpenAI) DescribeAction(ctx context.Context, a ActionContext) (string, error) {
	if o.APIKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY missing")
	}

	sys := `
	You write vivid, PG Pokémon battle commentary for a SINGLE action.
	Constraints:
	- A few short sentences total (max ~80 words).
	- No game numbers, no damage math, no emojis.
	- Use the source Pokémon's typical look/feel (wings, flames, vines, armor-like hide, etc.) without inventing new anatomy.
	- Use the move description for flavor (what it does / how it looks).
	- If hints say missed, crit, or effectiveness, reflect it naturally.
	- If a stat hint is provided (e.g., "lowers Speed"), imply it (e.g., "slowing it down").
	- Avoid repetition across lines; vary verbs and imagery.
	Output strict JSON: {"description": "..."}
	`

	user := fmt.Sprintf(
		`Action:
	source_name=%q
	source_types=%v
	target_name=%q
	target_types=%v
	move_name=%q
	move_type=%q
	move_power=%d
	move_description=%q
	hints: effectiveness=%q

	Write ONLY JSON. No explanations.`,
		a.Source.Name, a.Source.Types,
		a.Target.Name, a.Target.Types,
		a.Move.Name, a.Move.Type, a.Move.Power, a.Move.Description,
		a.Effectiveness,
	)

	body, _ := json.Marshal(chatReq{
		Model:       o.Model, // e.g., "gpt-4o-mini"
		Temperature: 0.7,
		Messages:    []chatMsg{{Role: "system", Content: sys}, {Role: "user", Content: user}},
		ResponseFmt: &respFormat{Type: "json_object"},
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: o.Timeout}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("openai status %s: %s", res.Status, string(b))
	}

	var cr chatResp
	if err := json.NewDecoder(res.Body).Decode(&cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}

	var parsed aiJSON
	if err := json.Unmarshal([]byte(cr.Choices[0].Message.Content), &parsed); err != nil {
		return "", err
	}
	if parsed.Description == "" {
		return "", fmt.Errorf("empty description")
	}
	return parsed.Description, nil
}
