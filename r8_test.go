package r8_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jgrecu/go-r8"
)

// newTestCPU returns a CPU with the program assembled from tokens loaded into memory.
func newTestCPU(t testing.TB, tokens ...string) *r8.CPU {
	t.Helper()
	program, err := r8.Assemble(tokens)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	if err := cpu.LoadProgram(program); err != nil {
		t.Fatal(err)
	}
	return cpu
}

func TestNewInitialisesCPU(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	if cpu.PC != 0 {
		t.Errorf("after New, want pc == 0, got %d", cpu.PC)
	}
	got := cpu.Mem[0]
	if got != 0 {
		t.Errorf("after New, want Memory[0] == 0, got %d", got)
	}

	if cpu.A != 0 {
		t.Errorf("after New, want a == 0, got %d", cpu.A)
	}
}

func TestStepIncrementsPC(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "NOP", "NOP")
	cpu.Step()
	if cpu.PC != 1 {
		t.Errorf("want pc == 1, got %d", cpu.PC)
	}
	cpu.Step()
	if cpu.PC != 2 {
		t.Errorf("want pc == 2, got %d", cpu.PC)
	}
}

func TestStepReturnsFalseOnUnknownOpcode(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Mem[0] = 0xFF // not in instructions table

	if cpu.Step() {
		t.Errorf("Step() on unknown opcode should return false, got true")
	}
	if cpu.PC != 1 {
		t.Errorf("PC should be 1, got %d", cpu.PC)
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		tokens []string
		initA  byte
		initPC uint16
		wantA  byte
		wantPC uint16
	}{
		{"halt stops cpu", []string{"NOP", "HALT"}, 0, 0, 0, 2},
		{"inc increments A", []string{"INC", "HALT"}, 0, 0, 1, 2},
		{"dec decrements A", []string{"INC", "DEC", "HALT"}, 0, 0, 0, 3},
		{"inc wraps A from 255 to 0", []string{"INC", "HALT"}, 255, 0, 0, 2},
		{"dec wraps A from 0 to 255", []string{"DEC", "HALT"}, 0, 0, 255, 2},
		{"ld sets A", []string{"LD", "10", "HALT"}, 0, 0, 10, 3},
		{"pc wraps from 65535 to 0", []string{"NOP", "HALT"}, 0, 65535, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := newTestCPU(t, c.tokens...)
			cpu.A = c.initA
			cpu.PC = c.initPC
			cpu.Run()
			if cpu.A != c.wantA {
				t.Errorf("want A == %d, got %d", c.wantA, cpu.A)
			}
			if cpu.PC != c.wantPC {
				t.Errorf("want PC == %d, got %d", c.wantPC, cpu.PC)
			}
		})
	}
}

func TestLoadFileLoadsDataFromFileIntoMemory(t *testing.T) {
	t.Parallel()
	want := []byte{0x30, 0x30, 0x00}
	path := filepath.Join(t.TempDir(), "input.bin")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("failed setting up test file: %v", err)
	}
	cpu := r8.NewCPU()
	if err := cpu.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	got := cpu.Mem[0:len(want)]
	if !bytes.Equal(got, want) {
		t.Errorf("want %+v, got %+v", want, got)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	err := cpu.LoadFile(filepath.Join(t.TempDir(), "missing.bin"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want fs.ErrNotExist, got %v", err)
	}
}

func TestLoadFileProgramTooLarge(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "large.bin")
	if err := os.WriteFile(path, make([]byte, r8.MemSize+1), 0o644); err != nil {
		t.Fatalf("failed setting up test file: %v", err)
	}
	cpu := r8.NewCPU()
	err := cpu.LoadFile(path)
	if !errors.Is(err, r8.ErrProgramTooLarge) {
		t.Errorf("want ErrProgramTooLarge, got %v", err)
	}
}

func TestLoadProgramTooLarge(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	err := cpu.LoadProgram(make([]byte, r8.MemSize+1))
	if !errors.Is(err, r8.ErrProgramTooLarge) {
		t.Errorf("want ErrProgramTooLarge, got %v", err)
	}
}

func TestDisassembleUnknownOpcode(t *testing.T) {
	t.Parallel()
	data := []byte{0xFF, 0x01} // 0xFF = unknown, 0x01 = NOP
	dasm := r8.Disassemble(data)
	want := []string{"???", "nop"}
	if !slices.Equal(dasm, want) {
		t.Errorf("Disassemble(%v) = %v, want %v", data, dasm, want)
	}
}

func TestDisassembleTruncated(t *testing.T) {
	t.Parallel()
	// LD (0x10) expects 1 operand, but none provided
	data := []byte{0x10}
	dasm := r8.Disassemble(data)
	want := []string{"ld", "???"} // mnemonic + truncation marker
	if !slices.Equal(dasm, want) {
		t.Errorf("Disassemble(%v) = %v, want %v", data, dasm, want)
	}
}

func TestAssembleErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		tokens  []string
		wantErr error
	}{
		{"invalid operand", []string{"LD", "not_a_number"}, r8.ErrBadOperand},
		{"operand out of range", []string{"LD", "300"}, r8.ErrBadOperand},
		{"unknown instruction", []string{"FOOBAR"}, r8.ErrUnknownInstruction},
		{"missing operand", []string{"LD"}, r8.ErrMissingOperand},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := r8.Assemble(c.tokens)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("Assemble(%v): want %v, got %v", c.tokens, c.wantErr, err)
			}
		})
	}
}

func TestAssembleDisassemble(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		inTokens    []string
		wantAsm     []byte
		wantDasm    []string
		expectError bool
	}{
		{"empty", []string{}, []byte{}, []string{}, false},
		{"nop", []string{"NOP"}, []byte{0x01}, []string{"nop"}, false},
		{"inc", []string{"INC"}, []byte{0x30}, []string{"inc"}, false},
		{"dec", []string{"DEC"}, []byte{0x40}, []string{"dec"}, false},
		{"halt", []string{"HALT"}, []byte{0x00}, []string{"halt"}, false},
		{"ld 20", []string{"LD", "20"}, []byte{0x10, 0x14}, []string{"ld 20"}, false},
		{"mixedcase", []string{"nop", "Inc", "dEc", "HaLt"}, []byte{0x01, 0x30, 0x40, 0x00}, []string{"nop", "inc", "dec", "halt"}, false},
		{"sequence", []string{"INC", "INC", "DEC", "HALT"}, []byte{0x30, 0x30, 0x40, 0x00}, []string{"inc", "inc", "dec", "halt"}, false},
		{"unknown", []string{"FOOBAR"}, []byte{}, []string{}, true},
		{"unknown in sequence", []string{"INC", "INC", "FOO", "HALT"}, []byte{}, []string{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			asm, err := r8.Assemble(c.inTokens)
			if err != nil && !c.expectError {
				t.Fatal(err)
			}
			if c.expectError && err == nil {
				t.Fatal("want error, got nil")
			}
			if !bytes.Equal(asm, c.wantAsm) {
				t.Errorf("Assemble(%v)=%v want %v", c.inTokens, asm, c.wantAsm)
			}

			dasm := r8.Disassemble(asm)
			if !slices.Equal(dasm, c.wantDasm) {
				t.Errorf("roundtrip %v -> dasm=%#v want %#v", c.inTokens, dasm, c.wantDasm)
			}
		})
	}
}


