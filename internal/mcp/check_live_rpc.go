package mcp

import (
	"encoding/json"
	"strings"
)

// decodeRPCMessage validates one unambiguous JSON-RPC envelope before use.
func decodeRPCMessage(raw []byte) (*rpcMessage, error) {
	var fields map[string]json.RawMessage
	if len(raw) > liveMaxMessage || decodeJSONBounded(raw, &fields, contractJSONDepth+8) != nil || fields == nil {
		return nil, badReply("invalid JSON-RPC envelope")
	}
	if !exactLiveFields(fields, "jsonrpc", "id", "method", "result", "error") {
		return nil, badReply("ambiguous JSON-RPC fields")
	}
	var msg rpcMessage
	if json.Unmarshal(raw, &msg) != nil || msg.JSONRPC != "2.0" || fields["jsonrpc"] == nil {
		return nil, badReply("invalid JSON-RPC version or fields")
	}
	if _, method := fields["method"]; method {
		// A server may send its own requests (such as ping) while replying.
		// Probe transports ignore these, as before; they are never responses
		// to our outstanding request even when their IDs happen to match.
		if msg.Method == "" || fields["result"] != nil || fields["error"] != nil {
			return nil, badReply("invalid JSON-RPC request or notification")
		}
		return &msg, nil
	}
	result, hasResult := fields["result"]
	errorRaw, hasError := fields["error"]
	if len(fields["id"]) == 0 || string(msg.ID) == "null" || hasResult == hasError {
		return nil, badReply("invalid JSON-RPC response")
	}
	if hasResult && (len(result) == 0 || string(result) == "null") {
		return nil, badReply("invalid JSON-RPC result")
	}
	if hasError {
		var e map[string]json.RawMessage
		if json.Unmarshal(errorRaw, &e) != nil || e == nil || e["code"] == nil || e["message"] == nil || msg.Error == nil || !exactLiveFields(e, "code", "message", "data") || string(e["code"]) == "null" || string(e["message"]) == "null" {
			return nil, badReply("invalid JSON-RPC error")
		}
	}
	return &msg, nil
}

// encoding/json matches struct fields case-insensitively; the protocol does not.
func exactLiveFields(fields map[string]json.RawMessage, reserved ...string) bool {
	for key := range fields {
		for _, name := range reserved {
			if key != name && strings.EqualFold(key, name) {
				return false
			}
		}
	}
	return true
}
