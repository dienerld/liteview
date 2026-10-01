package viewer

import (
	"encoding/json"
	"errors"

	"liteview/internal/rows"
)

// MarshalError makes constraint violations reach the frontend as structured data
// (available as `error.cause` in JS). Other errors use Wails' default handling.
func MarshalError(err error) []byte {
	var ce *rows.ConstraintError
	if !errors.As(err, &ce) {
		return nil
	}
	cols := ce.Columns
	if cols == nil {
		cols = []string{}
	}
	b, jerr := json.Marshal(map[string]any{
		"type":    "constraint",
		"kind":    ce.Kind,
		"columns": cols,
		"message": ce.Msg,
	})
	if jerr != nil {
		return nil
	}
	return b
}
