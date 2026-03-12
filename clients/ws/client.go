// Package ws provides a WebSocket client for the Ozzie gateway.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/coder/websocket"

	wsprotocol "github.com/dohr-michael/ozzie/internal/infra/gateway/ws"
)

// DialOption configures the WS dial.
type DialOption func(*dialConfig)

type dialConfig struct {
	token string
}

// WithToken sets the bearer token for authentication.
func WithToken(token string) DialOption {
	return func(c *dialConfig) { c.token = token }
}

// Client is a WebSocket client for the Ozzie gateway.
type Client struct {
	conn      *websocket.Conn
	reqSeq    uint64
	ctx       context.Context
	cancel    context.CancelFunc
	SessionID string
}

// Dial connects to the gateway WebSocket endpoint.
func Dial(ctx context.Context, url string, opts ...DialOption) (*Client, error) {
	cfg := &dialConfig{}
	for _, o := range opts {
		o(cfg)
	}

	var wsOpts *websocket.DialOptions
	if cfg.token != "" {
		wsOpts = &websocket.DialOptions{
			HTTPHeader: http.Header{
				"Authorization": []string{"Bearer " + cfg.token},
			},
		}
	}

	conn, _, err := websocket.Dial(ctx, url, wsOpts)
	if err != nil {
		return nil, fmt.Errorf("ws dial: %w", err)
	}

	clientCtx, cancel := context.WithCancel(ctx)

	return &Client{
		conn:   conn,
		ctx:    clientCtx,
		cancel: cancel,
	}, nil
}

// sendFire marshals and sends a request frame (fire-and-forget).
func (c *Client) sendFire(method string, params any) error {
	seq := atomic.AddUint64(&c.reqSeq, 1)

	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", method, err)
		}
	}

	frame := wsprotocol.Frame{
		Type:   wsprotocol.FrameTypeRequest,
		ID:     fmt.Sprintf("req-%d", seq),
		Method: method,
		Params: rawParams,
	}

	data, err := wsprotocol.MarshalFrame(frame)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", method, err)
	}

	return c.conn.Write(c.ctx, websocket.MessageText, data)
}

// sendRequest sends a request and waits for the response.
func (c *Client) sendRequest(method string, params any) (wsprotocol.Frame, error) {
	if err := c.sendFire(method, params); err != nil {
		return wsprotocol.Frame{}, err
	}

	resp, err := c.ReadFrame()
	if err != nil {
		return wsprotocol.Frame{}, fmt.Errorf("read %s response: %w", method, err)
	}

	if resp.OK != nil && !*resp.OK {
		return resp, fmt.Errorf("%s failed: %s", method, resp.Error)
	}

	return resp, nil
}

// OpenSessionOpts holds parameters for opening or resuming a session.
type OpenSessionOpts struct {
	SessionID string `json:"session_id,omitempty"`
	RootDir   string `json:"root_dir,omitempty"`
}

// OpenSession sends an open_session request and returns the session ID.
// If opts.SessionID is empty, the server creates a new session.
// If opts.SessionID is non-empty, the server resumes that session.
func (c *Client) OpenSession(opts OpenSessionOpts) (string, error) {
	resp, err := c.sendRequest(string(wsprotocol.MethodOpenSession), opts)
	if err != nil {
		return "", err
	}

	var result struct {
		SessionID string `json:"session_id"`
	}
	if resp.Payload != nil {
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return "", fmt.Errorf("unmarshal session response: %w", err)
		}
	}

	c.SessionID = result.SessionID
	return result.SessionID, nil
}

// SendMessage sends a user message to the gateway.
func (c *Client) SendMessage(content string) error {
	return c.sendFire(string(wsprotocol.MethodSendMessage), map[string]string{"content": content})
}

// RespondToPrompt sends a prompt_response to confirm or deny a tool execution.
func (c *Client) RespondToPrompt(token string, cancelled bool) error {
	return c.sendFire(string(wsprotocol.MethodPromptResponse), map[string]any{
		"token":     token,
		"cancelled": cancelled,
	})
}

// RespondToPromptWithValue sends a prompt_response with a string value (for text/password prompts).
func (c *Client) RespondToPromptWithValue(token string, value string) error {
	return c.sendFire(string(wsprotocol.MethodPromptResponse), map[string]any{
		"token": token,
		"value": value,
	})
}

// RespondToPromptWithValues sends a prompt_response with multiple values (for multi-select prompts).
func (c *Client) RespondToPromptWithValues(token string, values []string) error {
	return c.sendFire(string(wsprotocol.MethodPromptResponse), map[string]any{
		"token":  token,
		"values": values,
	})
}

// AcceptAllTools sends an accept_all_tools request to enable auto-approval
// of all dangerous tools for the current session.
func (c *Client) AcceptAllTools() error {
	_, err := c.sendRequest(string(wsprotocol.MethodAcceptAllTools), nil)
	return err
}

// HistoryMessage is a message returned by LoadMessages.
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Ts      string `json:"ts"`
}

// LoadMessages fetches the last N messages from the session history.
func (c *Client) LoadMessages(limit int) ([]HistoryMessage, error) {
	resp, err := c.sendRequest(string(wsprotocol.MethodLoadMessages), map[string]int{"limit": limit})
	if err != nil {
		return nil, err
	}

	var msgs []HistoryMessage
	if resp.Payload != nil {
		if err := json.Unmarshal(resp.Payload, &msgs); err != nil {
			return nil, fmt.Errorf("unmarshal messages: %w", err)
		}
	}

	return msgs, nil
}

// ReadFrame reads the next frame from the connection.
func (c *Client) ReadFrame() (wsprotocol.Frame, error) {
	_, data, err := c.conn.Read(c.ctx)
	if err != nil {
		return wsprotocol.Frame{}, err
	}
	return wsprotocol.UnmarshalFrame(data)
}

// Close gracefully closes the connection.
func (c *Client) Close() error {
	c.cancel()
	return c.conn.Close(websocket.StatusNormalClosure, "bye")
}
