package common

import (
	"encoding/json"
)

func Marshal(v any, prefix [2]byte) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(prefix[:], b...), nil
}

func Unmarshal(b []byte, prefix [2]byte, v any) error {
	if len(b) < 2 {
		return nil
	}
	err := json.Unmarshal(b[2:], v)
	if err != nil {
		return err
	}
	if prefix == StatDBPrefix {
		//v = v.(*statistics.MainStats)
		return nil
	}

	return nil
}
