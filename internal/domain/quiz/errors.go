package quiz

import "errors"

var ErrNotFound = errors.New("rows not found")

var ErrCustom = errors.New("Custom error")

var ErrInvalidQuestionPosition = errors.New("invalid question position")
