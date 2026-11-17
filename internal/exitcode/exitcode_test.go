package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestFrom(t *testing.T) {
	base := errors.New("boom")
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"plain error exits 1", base, 1},
		{"coded error keeps its code", With(ResourceExists, base), ResourceExists},
		{"wrapped coded error keeps its code", fmt.Errorf("context: %w", With(ConstraintViolated, base)), ConstraintViolated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := From(tt.err); got != tt.want {
				t.Errorf("From() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWithKeepsTheMessageAndTheCause(t *testing.T) {
	base := errors.New("boom")
	err := With(GitOperationFailed, base)
	if err.Error() != "boom" {
		t.Errorf("Error() = %q, want %q", err.Error(), "boom")
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is does not find the wrapped error")
	}
}
