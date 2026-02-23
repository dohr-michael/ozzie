package models

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/config"
)

const (
	defaultAnthropicModel     = "claude-sonnet-4-6"
	defaultAnthropicMaxTokens = 4096
)

// AnthropicChatModel implements model.ToolCallingChatModel using Anthropic's SDK.
type AnthropicChatModel struct {
	client    anthropic.Client
	modelName string
	maxTokens int
	tools     []*schema.ToolInfo
}

// NewAnthropic creates a new Anthropic ToolCallingChatModel.
func NewAnthropic(ctx context.Context, cfg config.ProviderConfig, auth ResolvedAuth) (model.ToolCallingChatModel, error) {
	modelName := cfg.Model
	if modelName == "" {
		modelName = defaultAnthropicModel
	}
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultAnthropicMaxTokens
	}

	var opts []option.RequestOption

	// API key auth (x-api-key header) vs Bearer token auth (Authorization header)
	switch auth.Kind {
	case AuthBearerToken:
		opts = append(opts, option.WithAuthToken(auth.Value))
	default:
		opts = append(opts, option.WithAPIKey(auth.Value))
	}

	//opts = append(opts, option.WithHeader("anthropic-beta", "interleaved-thinking-2025-05-14"))

	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.Timeout.Duration() > 0 {
		opts = append(opts, option.WithRequestTimeout(cfg.Timeout.Duration()))
	} else {
		opts = append(opts, option.WithRequestTimeout(60*time.Second))
	}

	return &AnthropicChatModel{
		client:    anthropic.NewClient(opts...),
		modelName: modelName,
		maxTokens: maxTokens,
	}, nil
}

func (m *AnthropicChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, "Anthropic", components.ComponentOfChatModel)

	cbInput := &model.CallbackInput{
		Messages: messages,
		Tools:    m.tools,
		Config:   &model.Config{Model: m.modelName},
	}
	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	params := m.buildParams(messages, opts)
	resp, err := m.client.Messages.New(ctx, params)
	if err != nil {
		return nil, HandleError(err)
	}

	outMsg = m.convertResponse(resp)

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message: outMsg,
		Config:  cbInput.Config,
		TokenUsage: &model.TokenUsage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	})

	return outMsg, nil
}

func (m *AnthropicChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, "Anthropic", components.ComponentOfChatModel)

	cbInput := &model.CallbackInput{
		Messages: messages,
		Tools:    m.tools,
		Config:   &model.Config{Model: m.modelName},
	}
	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	params := m.buildParams(messages, opts)
	stream := m.client.Messages.NewStreaming(ctx, params)

	sr, sw := schema.Pipe[*model.CallbackOutput](10)
	go m.streamResponse(ctx, stream, sw, cbInput.Config)

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, sr)

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src *model.CallbackOutput) (*schema.Message, error) {
			if src.Message == nil {
				return nil, schema.ErrNoValue
			}
			return src.Message, nil
		})

	return outStream, nil
}

func (m *AnthropicChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &AnthropicChatModel{
		client:    m.client,
		modelName: m.modelName,
		maxTokens: m.maxTokens,
		tools:     tools,
	}, nil
}

func (m *AnthropicChatModel) buildParams(messages []*schema.Message, opts []model.Option) anthropic.MessageNewParams {
	options := model.GetCommonOptions(&model.Options{
		MaxTokens: &m.maxTokens,
	}, opts...)

	maxTokens := m.maxTokens
	if options.MaxTokens != nil && *options.MaxTokens > 0 {
		maxTokens = *options.MaxTokens
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(m.modelName),
		MaxTokens: int64(maxTokens),
	}

	var anthropicMsgs []anthropic.MessageParam
	for _, msg := range messages {
		switch msg.Role {
		case schema.System:
			params.System = append(params.System, anthropic.TextBlockParam{
				Text: msg.Content,
			})
		default:
			anthropicMsgs = append(anthropicMsgs, m.convertMessage(msg))
		}
	}
	params.Messages = anthropicMsgs

	if len(m.tools) > 0 {
		var anthropicTools []anthropic.ToolUnionParam
		for _, tool := range m.tools {
			inputSchema := m.convertToolSchema(tool)
			toolParam := anthropic.ToolUnionParamOfTool(inputSchema, tool.Name)
			if toolParam.OfTool != nil {
				toolParam.OfTool.Description = param.NewOpt(tool.Desc)
			}
			anthropicTools = append(anthropicTools, toolParam)
		}
		params.Tools = anthropicTools
	}

	return params
}

func (m *AnthropicChatModel) convertToolSchema(tool *schema.ToolInfo) anthropic.ToolInputSchemaParam {
	inputSchema := anthropic.ToolInputSchemaParam{}

	if tool.ParamsOneOf == nil {
		return inputSchema
	}

	jsonSchema, err := tool.ParamsOneOf.ToJSONSchema()
	if err != nil || jsonSchema == nil {
		return inputSchema
	}

	schemaBytes, err := json.Marshal(jsonSchema)
	if err != nil {
		return inputSchema
	}

	var schemaMap map[string]any
	if json.Unmarshal(schemaBytes, &schemaMap) != nil {
		return inputSchema
	}

	if props, ok := schemaMap["properties"]; ok {
		inputSchema.Properties = props
	}
	if req, ok := schemaMap["required"].([]any); ok {
		required := make([]string, 0, len(req))
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
		inputSchema.Required = required
	}

	return inputSchema
}

func (m *AnthropicChatModel) convertMessage(msg *schema.Message) anthropic.MessageParam {
	switch msg.Role {
	case schema.User:
		return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))

	case schema.Assistant:
		var blocks []anthropic.ContentBlockParamUnion
		if msg.Content != "" {
			blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
		}
		for _, tc := range msg.ToolCalls {
			var input any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = tc.Function.Arguments
			}
			blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Function.Name))
		}
		return anthropic.NewAssistantMessage(blocks...)

	case schema.Tool:
		return anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false))

	default:
		return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
	}
}

func (m *AnthropicChatModel) convertResponse(resp *anthropic.Message) *schema.Message {
	result := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     int(resp.Usage.InputTokens),
				CompletionTokens: int(resp.Usage.OutputTokens),
			},
		},
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			inputJSON, err := json.Marshal(block.Input)
			if err != nil {
				inputJSON = []byte("{}")
			}
			result.ToolCalls = append(result.ToolCalls, schema.ToolCall{
				ID: block.ID,
				Function: schema.FunctionCall{
					Name:      block.Name,
					Arguments: string(inputJSON),
				},
			})
		}
	}

	switch resp.StopReason {
	case anthropic.StopReasonEndTurn:
		result.ResponseMeta.FinishReason = "stop"
	case anthropic.StopReasonToolUse:
		result.ResponseMeta.FinishReason = "tool_calls"
	case anthropic.StopReasonMaxTokens:
		result.ResponseMeta.FinishReason = "length"
	default:
		result.ResponseMeta.FinishReason = "stop"
	}

	return result
}

func (m *AnthropicChatModel) streamResponse(ctx context.Context, stream *ssestream.Stream[anthropic.MessageStreamEventUnion], writer *schema.StreamWriter[*model.CallbackOutput], cfg *model.Config) {
	defer writer.Close()

	var currentToolCall *schema.ToolCall
	var toolArgsJSON strings.Builder
	var usage schema.TokenUsage
	var content strings.Builder

	send := func(msg *schema.Message, tu *model.TokenUsage, err error) bool {
		return writer.Send(&model.CallbackOutput{
			Message:    msg,
			Config:     cfg,
			TokenUsage: tu,
		}, err)
	}

	finalMsg := func() *schema.Message {
		return &schema.Message{
			Role:    schema.Assistant,
			Content: content.String(),
			ResponseMeta: &schema.ResponseMeta{
				Usage:        &usage,
				FinishReason: "stop",
			},
		}
	}

	for stream.Next() {
		select {
		case <-ctx.Done():
			send(finalMsg(), toModelTokenUsage(&usage), ctx.Err())
			return
		default:
		}

		event := stream.Current()

		switch event.Type {
		case "message_start":
			usage.PromptTokens = int(event.Message.Usage.InputTokens)

		case "content_block_start":
			cb := event.ContentBlock
			if cb.Type == "tool_use" {
				currentToolCall = &schema.ToolCall{
					ID: cb.ID,
					Function: schema.FunctionCall{
						Name: cb.Name,
					},
				}
				toolArgsJSON.Reset()
			}

		case "content_block_delta":
			delta := event.Delta
			if delta.Type == "text_delta" {
				content.WriteString(delta.Text)
				if send(&schema.Message{
					Role:    schema.Assistant,
					Content: delta.Text,
				}, nil, nil) {
					return
				}
			} else if delta.Type == "input_json_delta" {
				toolArgsJSON.WriteString(delta.PartialJSON)
			}

		case "content_block_stop":
			if currentToolCall != nil {
				currentToolCall.Function.Arguments = toolArgsJSON.String()
				if send(&schema.Message{
					Role:      schema.Assistant,
					ToolCalls: []schema.ToolCall{*currentToolCall},
				}, nil, nil) {
					return
				}
				currentToolCall = nil
			}

		case "message_delta":
			usage.CompletionTokens = int(event.Usage.OutputTokens)

		case "message_stop":
			send(finalMsg(), toModelTokenUsage(&usage), nil)
			return
		}
	}

	if err := stream.Err(); err != nil {
		send(finalMsg(), toModelTokenUsage(&usage), err)
		return
	}
}

func toModelTokenUsage(u *schema.TokenUsage) *model.TokenUsage {
	if u == nil {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.PromptTokens + u.CompletionTokens,
	}
}

var _ model.ToolCallingChatModel = (*AnthropicChatModel)(nil)
