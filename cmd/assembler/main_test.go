package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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
	src := writeTemp(t, "prog.src", "ld 0x03\ndec\nbne 0xFD\nhalt\n")
	var stdout bytes.Buffer
	if err := run([]string{"-o", "-", src}, &stdout); err != nil {
		t.Fatal(err)
	}
	want := []byte{r8.OpLd, 3, r8.OpDec, r8.OpBne, 0xFD, r8.OpHalt}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Errorf("want %v, got %v", want, stdout.Bytes())
	}
}

func TestRunWritesMachineCodeToFile(t *testing.T) {
	t.Parallel()
	src := writeTemp(t, "prog.src", "inc halt")
	out := filepath.Join(t.TempDir(), "prog.bin")
	if err := run([]string{"-o", out, src}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{r8.OpInc, r8.OpHalt}; !bytes.Equal(got, want) {
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
