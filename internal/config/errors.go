package config

import (
	"fmt"
	"strings"
)

type NotFoundError struct {
	Path string
	Err  error
}

func (e *NotFoundError) Error() string { return "config: no " + e.Path }

func (e *NotFoundError) Unwrap() error { return e.Err }

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	if len(e.Problems) == 1 {
		return "config: " + e.Problems[0]
	}

	var b strings.Builder

	_, err := fmt.Fprintf(&b, "config: %d problems:", len(e.Problems))

	if err != nil {
		return ""
	}

	for _, p := range e.Problems {
		b.WriteString("\n  " + p)
	}

	return b.String()
}
