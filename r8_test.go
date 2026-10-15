package r8_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

func TestCpuRun(t *testing.T) {
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
		{"jmp skips inc", []string{"JMP", "4", "INC", "HALT"}, 0, 0, 0, 5},
		{"jmp after ld", []string{"LD", "10", "JMP", "0x0006", "INC", "HALT"}, 0, 0, 10, 7},
		{"jmp into own operand", []string{"JMP", "2", "INC", "HALT"}, 0, 0, 0, 3},
		{"bra 0 falls through", []string{"BRA", "0", "INC", "HALT"}, 0, 0, 1, 4},
		{"bra 1 skips inc", []string{"BRA", "1", "INC", "HALT"}, 0, 0, 0, 4},
		{"bra skips several instructions", []string{"BRA", "2", "INC", "INC", "INC", "HALT"}, 0, 0, 1, 6},
		{"bra after ld", []string{"LD", "10", "BRA", "1", "INC", "HALT"}, 0, 0, 10, 6},
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
	want := "???\nnop"
	if dasm != want {
		t.Errorf("Disassemble(%v) = %q, want %q", data, dasm, want)
	}
}

func TestDisassembleTruncated(t *testing.T) {
	t.Parallel()
	// LD (0x10) expects 1 operand, but none provided
	data := []byte{0x10}
	dasm := r8.Disassemble(data)
	want := "ld\n???" // mnemonic + truncation marker
	if dasm != want {
		t.Errorf("Disassemble(%v) = %q, want %q", data, dasm, want)
	}
}

func TestDisassembleTruncatedJmp(t *testing.T) {
	t.Parallel()
	data := []byte{0xF7, 0x02} // JMP needs 2 operand bytes, only 1 present
	dasm := r8.Disassemble(data)
	want := "jmp\n???"
	if dasm != want {
		t.Errorf("Disassemble(%v) = %q, want %q", data, dasm, want)
	}
}

func TestDisassembleTruncatedBra(t *testing.T) {
	t.Parallel()
	data := []byte{0xF0} // BRA needs 1 operand byte, none present
	dasm := r8.Disassemble(data)
	want := "bra\n???"
	if dasm != want {
		t.Errorf("Disassemble(%v) = %q, want %q", data, dasm, want)
	}
}

func TestJmpOperandWrapsAroundMemory(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.PC = 0xFFFE
	cpu.Mem[0xFFFE] = r8.OpJmp
	cpu.Mem[0xFFFF] = 0x04 // lo
	cpu.Mem[0x0000] = 0x00 // hi, read after PC wraps
	cpu.Step()
	if cpu.PC != 4 {
		t.Errorf("want PC == 4, got %d", cpu.PC)
	}
}

func TestBraWrapsAroundMemory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		pc     uint16
		offset byte
		wantPC uint16
	}{
		{"operand read after PC wraps", 0xFFFF, 1, 2},
		{"target past end of memory", 0xFFF0, 0x20, 0x0012},
		{"max offset", 0xFFFE, 0xFF, 0x00FF},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := r8.NewCPU()
			cpu.PC = c.pc
			cpu.Mem[c.pc] = r8.OpBra
			cpu.Mem[c.pc+1] = c.offset
			cpu.Step()
			if cpu.PC != c.wantPC {
				t.Errorf("want PC == %#04x, got %#04x", c.wantPC, cpu.PC)
			}
		})
	}
}

func TestJmpInfiniteLoop(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "LD", "10", "INC", "JMP", "0x0002")
	for range 1 + 3*2 { // ld, then 3 iterations of inc+jmp
		cpu.Step()
	}
	if cpu.A != 13 {
		t.Errorf("want A == 13, got %d", cpu.A)
	}
	if cpu.PC != 2 {
		t.Errorf("want PC == 2, got %d", cpu.PC)
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
		{"jmp address out of range", []string{"JMP", "65536"}, r8.ErrBadOperand},
		{"jmp missing operand", []string{"JMP"}, r8.ErrMissingOperand},
		{"bra missing operand", []string{"BRA"}, r8.ErrMissingOperand},
		{"bra operand out of range", []string{"BRA", "256"}, r8.ErrBadOperand},
		{"bra invalid operand", []string{"BRA", "x"}, r8.ErrBadOperand},
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
		wantDasm    string
		expectError bool
	}{
		{"empty", []string{}, []byte{}, "", false},
		{"nop", []string{"NOP"}, []byte{0x01}, "nop", false},
		{"inc", []string{"INC"}, []byte{0x30}, "inc", false},
		{"dec", []string{"DEC"}, []byte{0x40}, "dec", false},
		{"halt", []string{"HALT"}, []byte{0x00}, "halt", false},
		{"ld 20", []string{"LD", "20"}, []byte{0x10, 0x14}, "ld 20", false},
		{"mixedcase", []string{"nop", "Inc", "dEc", "HaLt"}, []byte{0x01, 0x30, 0x40, 0x00}, "nop\ninc\ndec\nhalt", false},
		{"sequence", []string{"INC", "INC", "DEC", "HALT"}, []byte{0x30, 0x30, 0x40, 0x00}, "inc\ninc\ndec\nhalt", false},
		{"unknown", []string{"FOOBAR"}, []byte{}, "", true},
		{"unknown in sequence", []string{"INC", "INC", "FOO", "HALT"}, []byte{}, "", true},
		{"jmp 0x0002", []string{"JMP", "0x0002"}, []byte{0xF7, 0x02, 0x00}, "jmp 0x0002", false},
		{"jmp high byte", []string{"JMP", "0x1234"}, []byte{0xF7, 0x34, 0x12}, "jmp 0x1234", false},
		{"jmp decimal in, hex out", []string{"JMP", "256"}, []byte{0xF7, 0x00, 0x01}, "jmp 0x0100", false},
		{"bra 3", []string{"BRA", "3"}, []byte{0xF0, 0x03}, "bra 3", false},
		{"bra 0", []string{"BRA", "0"}, []byte{0xF0, 0x00}, "bra 0", false},
		{"bra hex in, decimal out", []string{"BRA", "0xFF"}, []byte{0xF0, 0xFF}, "bra 255", false},
		{"bra in sequence", []string{"BRA", "1", "INC", "HALT"}, []byte{0xF0, 0x01, 0x30, 0x00}, "bra 1\ninc\nhalt", false},
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
			if dasm != c.wantDasm {
				t.Errorf("roundtrip %v -> dasm=%#v want %#v", c.inTokens, dasm, c.wantDasm)
			}
		})
	}
}
