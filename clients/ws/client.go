// Package ws provides a WebSocket client for the Ozzie gateway.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/coder/websocket"

	wsprotocol "github.com/dohr-michael/ozzie/internal/gateway/ws"
)

// Client is a WebSocket client for the Ozzie gateway.
type Client struct {
	conn      *websocket.Conn
	reqSeq    uint64
	ctx       context.Context
	cancel    context.CancelFunc
	SessionID string
}

// Dial connects to the gateway WebSocket endpoint.
func Dial(ctx context.Context, url string) (*Client, error) {
	conn, _, err := websocket.Dial(ctx, url, nil)
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

// OpenSessionOpts holds parameters for opening or resuming a session.
type OpenSessionOpts struct {
	SessionID string `json:"session_id,omitempty"`
	RootDir   string `json:"root_dir,omitempty"`
}

// OpenSession sends an open_session request and returns the session ID.
// If opts.SessionID is empty, the server creates a new session.
// If opts.SessionID is non-empty, the server resumes that session.
func (c *Client) OpenSession(opts OpenSessionOpts) (string, error) {
	seq := atomic.AddUint64(&c.reqSeq, 1)

	params, _ := json.Marshal(opts)

	frame := wsprotocol.Frame{
		Type:   wsprotocol.FrameTypeRequest,
		ID:     fmt.Sprintf("req-%d", seq),
		Method: string(wsprotocol.MethodOpenSession),
		Params: params,
	}

	data, err := wsprotocol.MarshalFrame(frame)
	if err != nil {
		return "", fmt.Errorf("marshal open_session: %w", err)
	}

	if err := c.conn.Write(c.ctx, websocket.MessageText, data); err != nil {
		return "", fmt.Errorf("send open_session: %w", err)
	}

	// Read the response
	resp, err := c.ReadFrame()
	if err != nil {
		return "", fmt.Errorf("read open_session response: %w", err)
	}

	if resp.OK != nil && !*resp.OK {
		return "", fmt.Errorf("open_session failed: %s", resp.Error)
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
	seq := atomic.AddUint64(&c.reqSeq, 1)

	params, _ := json.Marshal(map[string]string{"content": content})

	frame := wsprotocol.Frame{
		Type:   wsprotocol.FrameTypeRequest,
		ID:     fmt.Sprintf("req-%d", seq),
		Method: string(wsprotocol.MethodSendMessage),
		Params: params,
	}

	data, err := wsprotocol.MarshalFrame(frame)
	if err != nil {
		return err
	}

	return c.conn.Write(c.ctx, websocket.MessageText, data)
}

// RespondToPrompt sends a prompt_response to confirm or deny a tool execution.
func (c *Client) RespondToPrompt(token string, cancelled bool) error {
	seq := atomic.AddUint64(&c.reqSeq, 1)

	params, _ := json.Marshal(map[string]any{
		"token":     token,
		"cancelled": cancelled,
	})

	frame := wsprotocol.Frame{
		Type:   wsprotocol.FrameTypeRequest,
		ID:     fmt.Sprintf("req-%d", seq),
		Method: string(wsprotocol.MethodPromptResponse),
		Params: params,
	}

	data, err := wsprotocol.MarshalFrame(frame)
	if err != nil {
		return fmt.Errorf("marshal prompt_response: %w", err)
	}

	return c.conn.Write(c.ctx, websocket.MessageText, data)
}

// AcceptAllTools sends an accept_all_tools request to enable auto-approval
// of all dangerous tools for the current session.
func (c *Client) AcceptAllTools() error {
	seq := atomic.AddUint64(&c.reqSeq, 1)

	frame := wsprotocol.Frame{
		Type:   wsprotocol.FrameTypeRequest,
		ID:     fmt.Sprintf("req-%d", seq),
		Method: string(wsprotocol.MethodAcceptAllTools),
	}

	data, err := wsprotocol.MarshalFrame(frame)
	if err != nil {
		return fmt.Errorf("marshal accept_all_tools: %w", err)
	}

	if err := c.conn.Write(c.ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("send accept_all_tools: %w", err)
	}

	resp, err := c.ReadFrame()
	if err != nil {
		return fmt.Errorf("read accept_all_tools response: %w", err)
	}

	if resp.OK != nil && !*resp.OK {
		return fmt.Errorf("accept_all_tools failed: %s", resp.Error)
	}

	return nil
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
