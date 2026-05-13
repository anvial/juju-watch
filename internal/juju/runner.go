package juju

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func NewExecRunner() ExecRunner {
	return ExecRunner{}
}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	output := stderr.String()
	if output == "" {
		output = stdout.String()
	}
	return stdout.Bytes(), classifyCommandError(ctx, err, output)
}

type ErrorKind string

const (
	ErrJujuMissing ErrorKind = "juju-not-installed"
	ErrNotLoggedIn ErrorKind = "not-logged-in"
	ErrModel       ErrorKind = "model-not-found"
	ErrTimeout     ErrorKind = "timeout"
	ErrCommand     ErrorKind = "command-failed"
	ErrInvalidJSON ErrorKind = "invalid-json"
	ErrGraphviz    ErrorKind = "graphviz-missing"
)

type CommandError struct {
	Kind   ErrorKind
	Output string
	Err    error
}

func (e CommandError) Error() string {
	if e.Output != "" {
		return string(e.Kind) + ": " + strings.TrimSpace(e.Output)
	}
	if e.Err != nil {
		return string(e.Kind) + ": " + e.Err.Error()
	}
	return string(e.Kind)
}

func (e CommandError) Unwrap() error {
	return e.Err
}

func classifyCommandError(ctx context.Context, err error, output string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return CommandError{Kind: ErrTimeout, Output: output, Err: ctx.Err()}
	}
	if errors.Is(err, exec.ErrNotFound) {
		return CommandError{Kind: ErrJujuMissing, Output: output, Err: err}
	}
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "not logged") || strings.Contains(lower, "please login") || strings.Contains(lower, "not authenticated"):
		return CommandError{Kind: ErrNotLoggedIn, Output: output, Err: err}
	case strings.Contains(lower, "model") && (strings.Contains(lower, "not found") || strings.Contains(lower, "does not exist")):
		return CommandError{Kind: ErrModel, Output: output, Err: err}
	default:
		return CommandError{Kind: ErrCommand, Output: output, Err: err}
	}
}
