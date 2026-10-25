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
	bin := writeProgram(t, []byte{r8.OpLdRN, 7, r8.OpIncR, r8.OpHalt})
	var out bytes.Buffer
	if err := run([]string{bin}, strings.NewReader("\n\n\n\n\n"), &out); err != nil {
		t.Fatal(err)
	}
	want := `CPU state: PC: 0000  A: 00  PS: --------  Next op: LD A, 0x07` +
		`CPU state: PC: 0002  A: 07  PS: --------  Next op: INC A` +
		`CPU state: PC: 0003  A: 08  PS: --------  Next op: HALT` +
		`CPU state: PC: 0004  A: 08  PS: --------  Next op: halted` + "\n"
	if out.String() != want {
		t.Errorf("want:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestRunShowsFlags(t *testing.T) {
	t.Parallel()
	bin := writeProgram(t, []byte{r8.OpLdRN, 0x00, r8.OpLdRN, 0x80, r8.OpHalt})
	var out bytes.Buffer
	if err := run([]string{bin}, strings.NewReader("\n\n\n"), &out); err != nil {
		t.Fatal(err)
	}
	want := `CPU state: PC: 0000  A: 00  PS: --------  Next op: LD A, 0x00` +
		`CPU state: PC: 0002  A: 00  PS: ------Z-  Next op: LD A, 0x80` +
		`CPU state: PC: 0004  A: 80  PS: N-------  Next op: HALT` +
		`CPU state: PC: 0005  A: 80  PS: N-------  Next op: halted` + "\n"
	if out.String() != want {
		t.Errorf("want:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestRunStopsWhenInputEnds(t *testing.T) {
	t.Parallel()
	bin := writeProgram(t, []byte{r8.OpIncR, r8.OpIncR, r8.OpHalt})
	var out bytes.Buffer
	if err := run([]string{bin}, strings.NewReader("\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `CPU state: PC: 0001  A: 01  PS: --------  Next op: INC A`) {
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
