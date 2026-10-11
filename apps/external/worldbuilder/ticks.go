package worldbuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

var decimalTick = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// Accept exact decimal strings from JavaScript as well as the original numeric
// wire representation. Marshal remains unchanged: history and request hashes
// continue to use int64 JSON numbers, regardless of the input representation.
func parseTick(raw json.RawMessage) (*int64, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	text := string(raw)
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
	}
	if !decimalTick.MatchString(text) {
		return nil, fmt.Errorf("fictional time must be a signed decimal integer")
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("fictional time exceeds signed 64-bit range")
	}
	return &n, nil
}

func (r *Record) UnmarshalJSON(raw []byte) error {
	type plain Record
	var value plain
	wire := struct {
		*plain
		From  json.RawMessage `json:"from"`
		Until json.RawMessage `json:"until"`
	}{plain: &value}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&wire); err != nil {
		return err
	}
	var err error
	if value.From, err = parseTick(wire.From); err != nil {
		return err
	}
	if value.Until, err = parseTick(wire.Until); err != nil {
		return err
	}
	*r = Record(value)
	return nil
}

func (r *toolRequest) UnmarshalJSON(raw []byte) error {
	type plain toolRequest
	var value plain
	wire := struct {
		*plain
		At json.RawMessage `json:"at"`
	}{plain: &value}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&wire); err != nil {
		return err
	}
	var err error
	if value.At, err = parseTick(wire.At); err != nil {
		return err
	}
	*r = toolRequest(value)
	return nil
}
