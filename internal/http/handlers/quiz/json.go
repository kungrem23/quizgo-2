package quizhandler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var errMultipleJSONValues = errors.New("request body must contain exactly one JSON value")

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errMultipleJSONValues
		}
		return err
	}
	return nil
}
