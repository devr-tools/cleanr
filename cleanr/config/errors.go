package config

import (
	"fmt"
	"strings"
)

type FieldError struct {
	Path    string
	Message string
	Hint    string
}

func (e FieldError) Error() string {
	if strings.TrimSpace(e.Hint) == "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s. Fix: %s", e.Path, e.Message, e.Hint)
}

type ValidationErrors struct {
	Errors []FieldError
}

func (v *ValidationErrors) Add(path, message, hint string) {
	v.Errors = append(v.Errors, FieldError{
		Path:    path,
		Message: message,
		Hint:    hint,
	})
}

func (v ValidationErrors) HasAny() bool {
	return len(v.Errors) > 0
}

func (v ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return ""
	}
	if len(v.Errors) == 1 {
		return "invalid config: " + v.Errors[0].Error()
	}

	var b strings.Builder
	b.WriteString("invalid config:")
	for _, err := range v.Errors {
		b.WriteString("\n- ")
		b.WriteString(err.Error())
	}
	return b.String()
}
