package agent

import (
	"encoding/json"
	"errors"
)

var (
	errUnexpected   = errors.New("unexpected message type")
	errAuthRejected = errors.New("auth rejected")
	errNoConn       = errors.New("no active connection")
)

func marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
