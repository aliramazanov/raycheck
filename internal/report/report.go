package report

import "github.com/aliramazanov/raycheck/internal/measure"

type Report struct {
	Name         string
	Result       measure.Result
	HasSensitive bool

	SuppressionDeclared bool

	Strict bool
}
