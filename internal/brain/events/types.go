package events

import (
	"encoding/base64"
	"encoding/json"
)

// ResumeToken encapsulates info needed to resume an interrupted operation.
type ResumeToken struct {
	CheckpointID   string   `json:"c"`
	InterruptAddrs []string `json:"a"`
}

// EncodeResumeToken creates an opaque string token.
func EncodeResumeToken(checkpointID string, interruptAddrs []string) string {
	token := ResumeToken{
		CheckpointID:   checkpointID,
		InterruptAddrs: interruptAddrs,
	}
	data, _ := json.Marshal(token)
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeResumeToken extracts checkpoint ID and interrupt addresses.
func DecodeResumeToken(tokenStr string) (checkpointID string, interruptAddrs []string, err error) {
	data, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err != nil {
		return "", nil, err
	}
	var token ResumeToken
	if err := json.Unmarshal(data, &token); err != nil {
		return "", nil, err
	}
	return token.CheckpointID, token.InterruptAddrs, nil
}
