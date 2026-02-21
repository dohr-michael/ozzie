package plugins

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	extism "github.com/extism/go-sdk"

	"github.com/dohr-michael/ozzie/internal/events"
)

// KVStore is a per-plugin in-memory key-value store.
type KVStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewKVStore creates a new empty KV store.
func NewKVStore() *KVStore {
	return &KVStore{data: make(map[string][]byte)}
}

// Get returns the value for a key, or nil if not found.
func (s *KVStore) Get(key string) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// Set stores a value for a key.
func (s *KVStore) Set(key string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// hostLogMessage is the JSON structure for ozzie.log calls.
type hostLogMessage struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// hostKVRequest is the JSON structure for ozzie.kv_set calls.
type hostKVRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// hostEmitEvent is the JSON structure for ozzie.emit_event calls.
type hostEmitEvent struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// NewHostFunctions creates the standard Ozzie host functions for a plugin.
// All functions live in the "ozzie" namespace.
func NewHostFunctions(bus *events.Bus, kv *KVStore, pluginConfig map[string]string) []extism.HostFunction {
	var fns []extism.HostFunction

	// ozzie.log — structured logging from plugin
	logFn := extism.NewHostFunctionWithStack(
		"log",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			input, err := p.ReadBytes(stack[0])
			if err != nil {
				slog.Error("host: failed to read log input", "error", err)
				return
			}
			var msg hostLogMessage
			if err := json.Unmarshal(input, &msg); err != nil {
				slog.Warn("host: invalid log message", "raw", string(input))
				return
			}
			switch msg.Level {
			case "debug":
				slog.Debug("plugin", "msg", msg.Message)
			case "warn":
				slog.Warn("plugin", "msg", msg.Message)
			case "error":
				slog.Error("plugin", "msg", msg.Message)
			default:
				slog.Info("plugin", "msg", msg.Message)
			}
		},
		[]extism.ValueType{extism.ValueTypePTR},
		nil,
	)
	logFn.SetNamespace("ozzie")
	fns = append(fns, logFn)

	// ozzie.kv_get — read from per-plugin KV store
	kvGetFn := extism.NewHostFunctionWithStack(
		"kv_get",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			key, err := p.ReadString(stack[0])
			if err != nil {
				slog.Error("host: kv_get read key", "error", err)
				stack[0] = 0
				return
			}
			value := kv.Get(key)
			if value == nil {
				value = []byte("{}")
			}
			offset, err := p.WriteBytes(value)
			if err != nil {
				slog.Error("host: kv_get write result", "error", err)
				stack[0] = 0
				return
			}
			stack[0] = offset
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypePTR},
	)
	kvGetFn.SetNamespace("ozzie")
	fns = append(fns, kvGetFn)

	// ozzie.kv_set — write to per-plugin KV store
	kvSetFn := extism.NewHostFunctionWithStack(
		"kv_set",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			input, err := p.ReadBytes(stack[0])
			if err != nil {
				slog.Error("host: kv_set read input", "error", err)
				return
			}
			var req hostKVRequest
			if err := json.Unmarshal(input, &req); err != nil {
				slog.Error("host: kv_set parse", "error", err)
				return
			}
			kv.Set(req.Key, []byte(req.Value))
		},
		[]extism.ValueType{extism.ValueTypePTR},
		nil,
	)
	kvSetFn.SetNamespace("ozzie")
	fns = append(fns, kvSetFn)

	// ozzie.emit_event — publish an event on the bus
	emitFn := extism.NewHostFunctionWithStack(
		"emit_event",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			input, err := p.ReadBytes(stack[0])
			if err != nil {
				slog.Error("host: emit_event read", "error", err)
				return
			}
			var ev hostEmitEvent
			if err := json.Unmarshal(input, &ev); err != nil {
				slog.Error("host: emit_event parse", "error", err)
				return
			}
			bus.Publish(events.NewEvent(events.EventType(ev.Type), "plugin", ev.Payload))
		},
		[]extism.ValueType{extism.ValueTypePTR},
		nil,
	)
	emitFn.SetNamespace("ozzie")
	fns = append(fns, emitFn)

	// ozzie.get_config — read a plugin config value
	getConfigFn := extism.NewHostFunctionWithStack(
		"get_config",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			key, err := p.ReadString(stack[0])
			if err != nil {
				slog.Error("host: get_config read key", "error", err)
				stack[0] = 0
				return
			}
			value := pluginConfig[key]
			offset, err := p.WriteString(value)
			if err != nil {
				slog.Error("host: get_config write result", "error", err)
				stack[0] = 0
				return
			}
			stack[0] = offset
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypePTR},
	)
	getConfigFn.SetNamespace("ozzie")
	fns = append(fns, getConfigFn)

	return fns
}
