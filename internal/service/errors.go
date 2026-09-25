package service

import "errors"

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrAmbiguousSearch = errors.New("ambiguous name search")
	ErrNotFound        = errors.New("not found")
)
