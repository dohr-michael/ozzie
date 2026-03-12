package events

import "encoding/json"

// PromptType indicates the type of input expected.
type PromptType string

const (
	PromptTypeText    PromptType = "text"
	PromptTypeSelect  PromptType = "select"
	PromptTypeMulti   PromptType = "multi"
	PromptTypeConfirm  PromptType = "confirm"
	PromptTypePassword PromptType = "password"
)

// PromptOption represents a selectable option.
type PromptOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}

// PromptConfig holds configuration for any prompt type.
type PromptConfig struct {
	Type        PromptType     `json:"type"`
	Field       string         `json:"field"`
	Label       string         `json:"label"`
	Required    bool           `json:"required"`
	Default     any            `json:"default,omitempty"`
	ResumeToken string         `json:"resume_token"`
	Placeholder string         `json:"placeholder,omitempty"`
	Validation  string         `json:"validation,omitempty"`
	MaxLength   int            `json:"max_length,omitempty"`
	Options     []PromptOption `json:"options,omitempty"`
	MinSelect   int            `json:"min_select,omitempty"`
	MaxSelect   int            `json:"max_select,omitempty"`
	HelpText    string         `json:"help_text,omitempty"`
	Width       int            `json:"width,omitempty"`
}

// PromptResponse holds the user's response to a prompt.
type PromptResponse struct {
	Field       string `json:"field"`
	Value       any    `json:"value"`
	ResumeToken string `json:"resume_token"`
	Cancelled   bool   `json:"cancelled"`
}

func NewTextPromptConfig(field, label string, required bool) *PromptConfig {
	return &PromptConfig{
		Type:     PromptTypeText,
		Field:    field,
		Label:    label,
		Required: required,
	}
}

func (c *PromptConfig) WithPlaceholder(placeholder string) *PromptConfig {
	c.Placeholder = placeholder
	return c
}

func (c *PromptConfig) WithValidation(pattern string) *PromptConfig {
	c.Validation = pattern
	return c
}

func (c *PromptConfig) WithDefault(value any) *PromptConfig {
	c.Default = value
	return c
}

func (c *PromptConfig) WithResumeToken(token string) *PromptConfig {
	c.ResumeToken = token
	return c
}

func (c *PromptConfig) WithOptions(options []PromptOption) *PromptConfig {
	c.Options = options
	return c
}

func (c *PromptConfig) WithHelpText(text string) *PromptConfig {
	c.HelpText = text
	return c
}

func NewPasswordPromptConfig(field, label string) *PromptConfig {
	return &PromptConfig{
		Type:     PromptTypePassword,
		Field:    field,
		Label:    label,
		Required: true,
	}
}

func NewSelectPromptConfig(field, label string, options []PromptOption, required bool) *PromptConfig {
	return &PromptConfig{
		Type:     PromptTypeSelect,
		Field:    field,
		Label:    label,
		Options:  options,
		Required: required,
	}
}

func NewMultiPromptConfig(field, label string, options []PromptOption, minSelect, maxSelect int) *PromptConfig {
	return &PromptConfig{
		Type:      PromptTypeMulti,
		Field:     field,
		Label:     label,
		Options:   options,
		MinSelect: minSelect,
		MaxSelect: maxSelect,
		Required:  minSelect > 0,
	}
}

func (c *PromptConfig) ToPayload() map[string]any {
	var payload map[string]any
	data, _ := json.Marshal(c)
	_ = json.Unmarshal(data, &payload)
	return payload
}

func ParsePromptConfig(payload map[string]any) *PromptConfig {
	config := &PromptConfig{}
	data, _ := json.Marshal(payload)
	_ = json.Unmarshal(data, config)
	return config
}

func ParsePromptResponse(payload map[string]any) *PromptResponse {
	response := &PromptResponse{}
	data, _ := json.Marshal(payload)
	_ = json.Unmarshal(data, response)
	return response
}

func (r *PromptResponse) ToPayload() map[string]any {
	var payload map[string]any
	data, _ := json.Marshal(r)
	_ = json.Unmarshal(data, &payload)
	return payload
}

func (r *PromptResponse) GetStringValue() string {
	if s, ok := r.Value.(string); ok {
		return s
	}
	return ""
}

func (r *PromptResponse) GetStringSliceValue() []string {
	if arr, ok := r.Value.([]string); ok {
		return arr
	}
	if arr, ok := r.Value.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
