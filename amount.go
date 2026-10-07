package einvoice

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Amount is a JSON value the API types as `string | number`: a quantity, a unit price, an
// allowance. It keeps the exact bytes the server sent, so a decimal string is never rounded through
// a float: use String for the text and Float64 when a number is wanted. Build one with AmountString
// (the form the API prefers for money) or AmountNumber. The zero value marshals as JSON null.
type Amount struct {
	raw json.RawMessage
}

// AmountString makes an Amount that marshals as the JSON string s, e.g. AmountString("25000.00").
func AmountString(s string) Amount {
	b, _ := json.Marshal(s)
	return Amount{raw: b}
}

// AmountNumber makes an Amount that marshals as a JSON number.
func AmountNumber(f float64) Amount {
	return Amount{raw: json.RawMessage(strconv.FormatFloat(f, 'f', -1, 64))}
}

// IsString reports whether the value is a JSON string (false for a number or the zero value).
func (a Amount) IsString() bool {
	return len(a.raw) > 0 && a.raw[0] == '"'
}

// IsZero reports whether the Amount carries no value (the zero value, or JSON null).
func (a Amount) IsZero() bool {
	return len(a.raw) == 0
}

// String returns the value as text: the string's content, or the number's digits exactly as sent.
// The zero value is "".
func (a Amount) String() string {
	if a.IsString() {
		var s string
		if err := json.Unmarshal(a.raw, &s); err == nil {
			return s
		}
	}
	return string(a.raw)
}

// Float64 parses the value as a number, whether it was sent as a string or a number.
func (a Amount) Float64() (float64, error) {
	if a.IsZero() {
		return 0, errors.New("einvoice: Amount has no value")
	}
	return strconv.ParseFloat(a.String(), 64)
}

// Raw returns the exact JSON bytes (nil for the zero value).
func (a Amount) Raw() json.RawMessage {
	return a.raw
}

// MarshalJSON writes the raw bytes as they were given or received; the zero value is null.
func (a Amount) MarshalJSON() ([]byte, error) {
	if a.IsZero() {
		return []byte("null"), nil
	}
	return a.raw, nil
}

// UnmarshalJSON accepts a JSON string, a JSON number or null.
func (a *Amount) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		a.raw = nil
		return nil
	}
	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("einvoice: Amount: %w", err)
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if _, err := strconv.ParseFloat(string(data), 64); err != nil {
			return fmt.Errorf("einvoice: Amount: %q is not a JSON number", data)
		}
	default:
		return fmt.Errorf("einvoice: Amount: expected a JSON string or number, got %s", data)
	}
	a.raw = append(json.RawMessage(nil), data...)
	return nil
}
