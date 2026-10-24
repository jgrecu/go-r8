package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	r8 "github.com/jgrecu/go-r8"
)

func writeProgram(t *testing.T, program []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prog.bin")
	if err := os.WriteFile(path, program, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunStepsOncePerInputLineAndShowsFinalState(t *testing.T) {
	t.Parallel()
	bin := writeProgram(t, []byte{r8.OpLd, 7, r8.OpInc, r8.OpHalt})
	var out bytes.Buffer
	if err := run([]string{bin}, strings.NewReader("\n\n\n\n\n"), &out); err != nil {
		t.Fatal(err)
	}
	want := header + "\n" +
		`next "LD 7"       => 0000 > 0000000` +
		`next "INC"        => 0002 > 0000007` +
		`next "HALT"       => 0003 > 0000008` +
		`halted            => 0004 > 0000008` + "\n"
	if out.String() != want {
		t.Errorf("want:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestRunStopsWhenInputEnds(t *testing.T) {
	t.Parallel()
	bin := writeProgram(t, []byte{r8.OpInc, r8.OpInc, r8.OpHalt})
	var out bytes.Buffer
	if err := run([]string{bin}, strings.NewReader("\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `next "INC"        => 0001 > 0000001`) {
		t.Errorf("want state after one step, got:\n%s", out.String())
	}
}

func TestRunReturnsIllegalInstructionError(t *testing.T) {
	t.Parallel()
	bin := writeProgram(t, []byte{0xFF})
	err := run([]string{bin}, strings.NewReader("\n"), &bytes.Buffer{})
	if _, ok := errors.AsType[*r8.IllegalInstructionError](err); !ok {
		t.Errorf("want *IllegalInstructionError, got %v", err)
	}
}

func TestRunRequiresOneProgramFile(t *testing.T) {
	t.Parallel()
	if err := run(nil, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Error("want usage error, got nil")
	}
}
