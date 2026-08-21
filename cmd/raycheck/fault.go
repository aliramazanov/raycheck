package main

import (
	"errors"
	"fmt"

	"github.com/aliramazanov/raycheck/internal/obs"
)

type fault struct {
	code int
	err  error
}

func (f fault) Error() string { return f.err.Error() }

func (f fault) Unwrap() error { return f.err }

func badUsage(format string, args ...any) fault {
	return fault{code: exitBadUsage, err: fmt.Errorf(format, args...)}
}

func inputFault(err error) fault { return fault{code: exitInput, err: err} }

func fail(rec *obs.Recorder, err error) int {
	code := exitInput

	if f, ok := errors.AsType[fault](err); ok {
		code = f.code
	}

	rec.Fatal(err.Error(), "exit_code", code)

	return code
}
