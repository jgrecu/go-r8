package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	r8 "github.com/jgrecu/go-r8"
)

func writeTemp(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunWritesSourceToStdout(t *testing.T) {
	t.Parallel()
	bin := writeTemp(t, "prog.bin", []byte{r8.OpLdRN, 3, r8.OpDecR, r8.OpBne, 0xFD, r8.OpHalt})
	var stdout bytes.Buffer
	if err := run([]string{"-o", "-", bin}, &stdout); err != nil {
		t.Fatal(err)
	}
	if want := "ld a, 0x03\ndec a\nbne 0xFD ; bne -3\nhalt\n"; stdout.String() != want {
		t.Errorf("want %q, got %q", want, stdout.String())
	}
}

func TestRunWritesSourceToFile(t *testing.T) {
	t.Parallel()
	bin := writeTemp(t, "prog.bin", []byte{r8.OpIncR, r8.OpHalt})
	out := filepath.Join(t.TempDir(), "prog.src")
	var stdout bytes.Buffer
	if err := run([]string{"-o", out, bin}, &stdout); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "inc a\nhalt\n"; string(got) != want {
		t.Errorf("want %q, got %q", want, got)
	}
	if stdout.Len() != 0 {
		t.Errorf("want nothing on stdout when writing a file, got %q", stdout.String())
	}
}

func TestRunRequiresOneInputFile(t *testing.T) {
	t.Parallel()
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Error("want usage error, got nil")
	}
}
