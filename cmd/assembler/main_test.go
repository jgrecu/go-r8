package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	r8 "github.com/jgrecu/go-r8"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunWritesMachineCodeToStdout(t *testing.T) {
	t.Parallel()
	src := writeTemp(t, "prog.src", "ld a, 0x03\ndec a\nbne 0xFD\nhalt\n")
	var stdout bytes.Buffer
	if err := run([]string{"-o", "-", src}, &stdout); err != nil {
		t.Fatal(err)
	}
	want := []byte{r8.OpLdRN, 3, r8.OpDecR, r8.OpBne, 0xFD, r8.OpHalt}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Errorf("want %v, got %v", want, stdout.Bytes())
	}
}

func TestRunWritesMachineCodeToFile(t *testing.T) {
	t.Parallel()
	src := writeTemp(t, "prog.src", "inc a halt")
	out := filepath.Join(t.TempDir(), "prog.bin")
	if err := run([]string{"-o", out, src}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{r8.OpIncR, r8.OpHalt}; !bytes.Equal(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestRunReportsAssemblyErrors(t *testing.T) {
	t.Parallel()
	src := writeTemp(t, "prog.src", "foo")
	err := run([]string{"-o", "-", src}, &bytes.Buffer{})
	if !errors.Is(err, r8.ErrUnknownInstruction) {
		t.Errorf("want ErrUnknownInstruction, got %v", err)
	}
}

func TestRunRequiresOneSourceFile(t *testing.T) {
	t.Parallel()
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Error("want usage error, got nil")
	}
}

func TestRunRejectsTwoPositionalArgs(t *testing.T) {
	t.Parallel()
	err := run([]string{"a.src", "b.src"}, &bytes.Buffer{})
	if want := "usage: assembler [-o output] source"; err == nil || err.Error() != want {
		t.Errorf("want %q, got %v", want, err)
	}
}

func TestRunRejectsBadFlag(t *testing.T) {
	t.Parallel()
	err := run([]string{"-bogus", "prog.src"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("want an error for an undefined flag, got nil")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined: -bogus") {
		t.Errorf("want a flag error, got %v", err)
	}
}

func TestRunMissingInputFile(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing.src")
	if err := run([]string{"-o", "-", missing}, &bytes.Buffer{}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want fs.ErrNotExist, got %v", err)
	}
}

func TestRunCannotWriteToNonexistentDirectory(t *testing.T) {
	t.Parallel()
	src := writeTemp(t, "prog.src", "halt")
	out := filepath.Join(t.TempDir(), "nope", "prog.bin")
	if err := run([]string{"-o", out, src}, &bytes.Buffer{}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want fs.ErrNotExist, got %v", err)
	}
}
