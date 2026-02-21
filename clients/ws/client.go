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
	conn   *websocket.Conn
	reqSeq uint64
	ctx    context.Context
	cancel context.CancelFunc
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

// SendMessage sends a user message to the gateway.
func (c *Client) SendMessage(content string) error {
	seq := atomic.AddUint64(&c.reqSeq, 1)

	params, _ := json.Marshal(map[string]string{"content": content})

	frame := wsprotocol.Frame{
		Type:   wsprotocol.FrameTypeRequest,
		ID:     fmt.Sprintf("req-%d", seq),
		Method: "send_message",
		Params: params,
	}

	data, err := wsprotocol.MarshalFrame(frame)
	if err != nil {
		return err
	}

	return c.conn.Write(c.ctx, websocket.MessageText, data)
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
