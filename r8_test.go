package r8_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jgrecu/go-r8"
)

// assemble assembles tokens written on a single source line.
func assemble(tokens ...string) ([]byte, error) {
	return r8.Assemble(strings.Join(tokens, " "))
}

// newTestCPU returns a CPU with the program assembled from tokens loaded into memory.
func newTestCPU(t testing.TB, tokens ...string) *r8.CPU {
	t.Helper()
	program, err := assemble(tokens...)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	if err := cpu.LoadProgram(program, 0); err != nil {
		t.Fatal(err)
	}
	return cpu
}

// mustStep executes one instruction, failing the test if Step returns an error.
func mustStep(t testing.TB, cpu *r8.CPU) {
	t.Helper()
	if err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
}

// FuzzDisassembleAssemble checks that any machine code that disassembles
// cleanly assembles back to exactly the same bytes.
func FuzzDisassembleAssemble(f *testing.F) {
	f.Add([]byte{r8.OpLdRN, 3, r8.OpDecR, r8.OpBne, 0xFD, r8.OpHalt})
	f.Add([]byte{r8.OpJmp, 0x34, 0x12, r8.OpBra, 0x80})
	f.Fuzz(func(t *testing.T, code []byte) {
		src := r8.Disassemble(code)
		if strings.Contains(src, "???") {
			t.Skip()
		}
		got, err := r8.Assemble(src)
		if err != nil {
			t.Fatalf("Assemble(%q): %v", src, err)
		}
		if !bytes.Equal(got, code) {
			t.Errorf("round trip of %v via %q gave %v", code, src, got)
		}
	})
}

func TestNewInitialisesCPU(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	if cpu.PC != 0 {
		t.Errorf("after New, want pc == 0, got %d", cpu.PC)
	}
	got := cpu.Memory[0]
	if got != 0 {
		t.Errorf("after New, want Memory[0] == 0, got %d", got)
	}

	for r, v := range cpu.R {
		if v != 0 {
			t.Errorf("after New, want %v == 0, got %d", r8.Register(r), v)
		}
	}
	if cpu.Halted {
		t.Error("after New, want Halted == false, got true")
	}
}

// TestRegisterIDs checks the register IDs against the R8 manual's register ID
// encoding table.
func TestRegisterIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		reg  r8.Register
		id   byte
		name string
	}{
		{r8.A, 0x0, "a"},
	}
	if len(cases) != r8.NumRegisters {
		t.Fatalf("want %d registers, got %d", len(cases), r8.NumRegisters)
	}
	for _, c := range cases {
		if byte(c.reg) != c.id {
			t.Errorf("%s: want ID 0x%X, got 0x%X", c.name, c.id, byte(c.reg))
		}
		if got := c.reg.String(); got != c.name {
			t.Errorf("ID 0x%X: want name %q, got %q", c.id, c.name, got)
		}
	}
}

// TestRegisterInstructions checks that, for every register, the instructions
// that encode it in the opcode's low nibble assemble to base|ID, change only
// that register, and disassemble back to the same source.
func TestRegisterInstructions(t *testing.T) {
	t.Parallel()
	for r := range r8.Register(r8.NumRegisters) {
		t.Run(r.String(), func(t *testing.T) {
			t.Parallel()
			src := fmt.Sprintf("ld %v, 0x07\ninc %v\ninc %v\ndec %v\nhalt", r, r, r, r)
			program, err := r8.Assemble(src)
			if err != nil {
				t.Fatal(err)
			}
			id := byte(r)
			want := []byte{r8.OpLdRN | id, 0x07, r8.OpIncR | id, r8.OpIncR | id, r8.OpDecR | id, r8.OpHalt}
			if !bytes.Equal(program, want) {
				t.Errorf("Assemble(%q) = % X, want % X", src, program, want)
			}
			if got := r8.Disassemble(program); got != src {
				t.Errorf("Disassemble(% X) = %q, want %q", program, got, src)
			}

			cpu := r8.NewCPU()
			if err := cpu.LoadProgram(program, 0); err != nil {
				t.Fatal(err)
			}
			if err := cpu.Run(); err != nil {
				t.Fatal(err)
			}
			for other, v := range cpu.R {
				want := byte(0)
				if r8.Register(other) == r {
					want = 0x08
				}
				if v != want {
					t.Errorf("want %v == %d, got %d", r8.Register(other), want, v)
				}
			}
		})
	}
}

func TestRegisterStringUnknownID(t *testing.T) {
	t.Parallel()
	if got, want := r8.Register(0xC).String(), "Register(12)"; got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStepIncrementsPC(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "NOP", "NOP")
	mustStep(t, cpu)
	if cpu.PC != 1 {
		t.Errorf("want pc == 1, got %d", cpu.PC)
	}
	mustStep(t, cpu)
	if cpu.PC != 2 {
		t.Errorf("want pc == 2, got %d", cpu.PC)
	}
}

func TestStepReturnsIllegalInstructionErrorOnUnknownOpcode(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Memory[0] = 0xFF // not in instructions table

	err := cpu.Step()
	illegal, ok := errors.AsType[*r8.IllegalInstructionError](err)
	if !ok {
		t.Fatalf("want *IllegalInstructionError, got %v", err)
	}
	if illegal.Opcode != 0xFF {
		t.Errorf("want Opcode == 0xFF, got %#02x", illegal.Opcode)
	}
	if cpu.PC != 1 {
		t.Errorf("PC should be 1, got %d", cpu.PC)
	}
	if cpu.Halted {
		t.Error("want Halted == false after illegal instruction, got true")
	}
}

func TestStepSetsHaltedOnHalt(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "HALT")
	if err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if !cpu.Halted {
		t.Error("want Halted == true after HALT, got false")
	}
}

func TestStepDoesNothingWhenHalted(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "INC", "A")
	cpu.Halted = true
	if err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if cpu.PC != 0 {
		t.Errorf("want PC == 0, got %d", cpu.PC)
	}
	if cpu.R[r8.A] != 0 {
		t.Errorf("want A == 0, got %d", cpu.R[r8.A])
	}
}

func TestRunReturnsErrorOnIllegalInstruction(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Memory[0] = 0xFF
	err := cpu.Run()
	if _, ok := errors.AsType[*r8.IllegalInstructionError](err); !ok {
		t.Fatalf("want *IllegalInstructionError, got %v", err)
	}
}

func TestRunNReturnsErrStepLimitWhenProgramDoesNotHalt(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "INC", "A", "JMP", "0x0000")
	err := cpu.RunN(10)
	if !errors.Is(err, r8.ErrStepLimit) {
		t.Fatalf("want ErrStepLimit, got %v", err)
	}
	if cpu.R[r8.A] != 5 {
		t.Errorf("want A == 5 after 10 steps, got %d", cpu.R[r8.A])
	}
}

func TestRunNStopsAtHalt(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "INC", "A", "HALT", "INC", "A")
	if err := cpu.RunN(10); err != nil {
		t.Fatal(err)
	}
	if !cpu.Halted || cpu.R[r8.A] != 1 {
		t.Errorf("want Halted with A == 1, got Halted=%v A=%d", cpu.Halted, cpu.R[r8.A])
	}
}

func TestRunNReturnsIllegalInstructionError(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Memory[0] = 0xFF
	if _, ok := errors.AsType[*r8.IllegalInstructionError](cpu.RunN(10)); !ok {
		t.Fatal("want *IllegalInstructionError")
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
		{"inc increments A", []string{"INC", "A", "HALT"}, 0, 0, 1, 2},
		{"dec decrements A", []string{"INC", "A", "DEC", "A", "HALT"}, 0, 0, 0, 3},
		{"inc wraps A from 255 to 0", []string{"INC", "A", "HALT"}, 255, 0, 0, 2},
		{"dec wraps A from 0 to 255", []string{"DEC", "A", "HALT"}, 0, 0, 255, 2},
		{"ld sets A", []string{"LD", "A,", "0x0A", "HALT"}, 0, 0, 10, 3},
		{"pc wraps from 65535 to 0", []string{"NOP", "HALT"}, 0, 65535, 0, 0},
		{"jmp skips inc", []string{"JMP", "0x04", "INC", "A", "HALT"}, 0, 0, 0, 5},
		{"jmp after ld", []string{"LD", "A,", "0x0A", "JMP", "0x0006", "INC", "A", "HALT"}, 0, 0, 10, 7},
		{"jmp into own operand", []string{"JMP", "0x02", "INC", "A", "HALT"}, 0, 0, 0, 3},
		{"bra 0 falls through", []string{"BRA", "0x00", "INC", "A", "HALT"}, 0, 0, 1, 4},
		{"bra 1 skips inc", []string{"BRA", "0x01", "INC", "A", "HALT"}, 0, 0, 0, 4},
		{"bra skips several instructions", []string{"BRA", "0x02", "INC", "A", "INC", "A", "INC", "A", "HALT"}, 0, 0, 1, 6},
		{"bra after ld", []string{"LD", "A,", "0x0A", "BRA", "0x01", "INC", "A", "HALT"}, 0, 0, 10, 6},
		{"beq taken when A is zero", []string{"BEQ", "0x01", "INC", "A", "HALT"}, 0, 0, 0, 4},
		{"beq not taken when A is nonzero", []string{"LD", "A,", "0x05", "BEQ", "0x01", "INC", "A", "HALT"}, 0, 0, 6, 6},
		{"beq taken after dec reaches zero", []string{"LD", "A,", "0x01", "DEC", "A", "BEQ", "0x01", "INC", "A", "HALT"}, 0, 0, 0, 7},
		{"beq 0 falls through", []string{"BEQ", "0x00", "INC", "A", "HALT"}, 0, 0, 1, 4},
		{"bne taken when A is nonzero", []string{"LD", "A,", "0x05", "BNE", "0x01", "INC", "A", "HALT"}, 0, 0, 5, 6},
		{"bne not taken when A is zero", []string{"BNE", "0x01", "INC", "A", "HALT"}, 0, 0, 1, 4},
		{"bne not taken after dec reaches zero", []string{"LD", "A,", "0x01", "DEC", "A", "BNE", "0x01", "INC", "A", "HALT"}, 0, 0, 1, 7},
		{"bne 0 falls through", []string{"LD", "A,", "0x05", "BNE", "0x00", "INC", "A", "HALT"}, 0, 0, 6, 6},
		{"bne backward loops until A is zero", []string{"LD", "A,", "0x03", "DEC", "A", "BNE", "0xFD", "HALT"}, 0, 0, 0, 6},
		{"beq backward taken once", []string{"BRA", "0x01", "HALT", "LD", "A,", "0x00", "BEQ", "0xFB"}, 0, 0, 0, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := newTestCPU(t, c.tokens...)
			cpu.R[r8.A] = c.initA
			cpu.PC = c.initPC
			if err := cpu.Run(); err != nil {
				t.Fatal(err)
			}
			if !cpu.Halted {
				t.Error("want Halted == true after Run, got false")
			}
			if cpu.R[r8.A] != c.wantA {
				t.Errorf("want A == %d, got %d", c.wantA, cpu.R[r8.A])
			}
			if cpu.PC != c.wantPC {
				t.Errorf("want PC == %d, got %d", c.wantPC, cpu.PC)
			}
		})
	}
}

func TestLoadProgramTooLarge(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		size int
		addr uint16
	}{
		{"larger than memory at 0", r8.MemSize + 1, 0},
		{"one byte into reset vector at 0", int(r8.ResetVector) + 1, 0},
		{"at reset vector low byte", 1, r8.ResetVector},
		{"at reset vector high byte", 1, r8.ResetVector + 1},
		{"into reset vector at offset", 0x0F, 0xFFF0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := r8.NewCPU()
			cpu.PC = 0x1234
			err := cpu.LoadProgram(bytes.Repeat([]byte{r8.OpNop}, c.size), c.addr)
			if !errors.Is(err, r8.ErrProgramTooLarge) {
				t.Errorf("want ErrProgramTooLarge, got %v", err)
			}
			if *cpu != (r8.CPU{PC: 0x1234}) {
				t.Error("want CPU unchanged after ErrProgramTooLarge")
			}
		})
	}
}

func TestLoadProgramFillsMemoryExactly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		size int
		addr uint16
	}{
		{"all memory below reset vector at 0", int(r8.ResetVector), 0},
		{"last byte below reset vector", 1, r8.ResetVector - 1},
		{"up to reset vector at offset", 0x0E, 0xFFF0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := r8.NewCPU()
			if err := cpu.LoadProgram(make([]byte, c.size), c.addr); err != nil {
				t.Errorf("want nil error, got %v", err)
			}
		})
	}
}

func TestLoadProgramAtAddress(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	program := []byte{r8.OpLdRN, 7, r8.OpHalt}
	if err := cpu.LoadProgram(program, 0x0100); err != nil {
		t.Fatal(err)
	}
	if got := cpu.Memory[0x0100 : 0x0100+len(program)]; !bytes.Equal(got, program) {
		t.Errorf("want Memory[0x0100:] == %v, got %v", program, got)
	}
	if cpu.Memory[0] != 0 {
		t.Errorf("want Memory[0] untouched (0), got %#02x", cpu.Memory[0])
	}
}

func TestLoadProgramAtAddressRunsFromThatAddress(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	program := []byte{r8.OpLdRN, 7, r8.OpHalt}
	if err := cpu.LoadProgram(program, 0x0100); err != nil {
		t.Fatal(err)
	}
	if cpu.PC != 0x0100 {
		t.Errorf("want PC 0x0100 after load, got %#04x", cpu.PC)
	}
	if err := cpu.Run(); err != nil {
		t.Fatal(err)
	}
	if cpu.R[r8.A] != 7 {
		t.Errorf("want A 7, got %d", cpu.R[r8.A])
	}
	if cpu.PC != 0x0103 {
		t.Errorf("want PC 0x0103, got %#04x", cpu.PC)
	}
}

func TestLoadProgramWritesResetVector(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	if err := cpu.LoadProgram([]byte{r8.OpHalt}, 0xC000); err != nil {
		t.Fatal(err)
	}
	lo, hi := cpu.Memory[r8.ResetVector], cpu.Memory[r8.ResetVector+1]
	if lo != 0x00 || hi != 0xC0 {
		t.Errorf("want reset vector bytes 00 C0, got %02X %02X", lo, hi)
	}
}

func TestLoadProgramClearsHalted(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "HALT")
	mustStep(t, cpu)
	if err := cpu.LoadProgram([]byte{r8.OpNop}, 0x0100); err != nil {
		t.Fatal(err)
	}
	if cpu.Halted {
		t.Error("want Halted == false after LoadProgram, got true")
	}
}

func TestResetLoadsPCFromLittleEndianResetVector(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Memory[0xFFFE] = 0x34 // low byte
	cpu.Memory[0xFFFF] = 0x12 // high byte
	cpu.Reset()
	if cpu.PC != 0x1234 {
		t.Errorf("want PC == 0x1234, got %#04x", cpu.PC)
	}
}

func TestResetOfNewCPUSetsPCToZero(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.PC = 0x1234
	cpu.Reset()
	if cpu.PC != 0 {
		t.Errorf("want PC == 0 from zeroed reset vector, got %#04x", cpu.PC)
	}
}

func TestResetClearsHalted(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Halted = true
	cpu.Reset()
	if cpu.Halted {
		t.Error("want Halted == false after Reset, got true")
	}
}

func TestResetLeavesRegistersAndMemoryUnchanged(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.R[r8.A] = 0x42
	cpu.Memory[0x0100] = 0x99
	cpu.Memory[r8.ResetVector] = 0x00
	cpu.Memory[r8.ResetVector+1] = 0x01
	want := cpu.Memory
	cpu.Reset()
	if cpu.R[r8.A] != 0x42 {
		t.Errorf("want A == 0x42, got %#02x", cpu.R[r8.A])
	}
	if cpu.Memory != want {
		t.Error("want Memory unchanged after Reset")
	}
}

func TestResetRestartsProgram(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	if err := cpu.LoadProgram([]byte{r8.OpIncR | byte(r8.A), r8.OpHalt}, 0x0200); err != nil {
		t.Fatal(err)
	}
	if err := cpu.Run(); err != nil {
		t.Fatal(err)
	}
	cpu.Reset()
	if err := cpu.Run(); err != nil {
		t.Fatal(err)
	}
	if cpu.R[r8.A] != 2 {
		t.Errorf("want A == 2 after running twice, got %d", cpu.R[r8.A])
	}
	if cpu.PC != 0x0202 {
		t.Errorf("want PC == 0x0202, got %#04x", cpu.PC)
	}
}

func TestDisassembleInvalidInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"unknown opcode", []byte{0xFF, r8.OpNop}, "???\nnop"},
		{"ld missing operand", []byte{r8.OpLdRN}, "ld a, ???"},
		{"jmp missing operand byte", []byte{r8.OpJmp, 0x02}, "jmp ???"},
		{"bra missing operand", []byte{r8.OpBra}, "bra ???"},
		{"beq missing operand", []byte{r8.OpBeq}, "beq ???"},
		{"bne missing operand", []byte{r8.OpBne}, "bne ???"},
		{"truncated after valid code", []byte{r8.OpIncR, r8.OpLdRN}, "inc a\nld a, ???"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dasm := r8.Disassemble(c.data)
			if dasm != c.want {
				t.Errorf("Disassemble(%v) = %q, want %q", c.data, dasm, c.want)
			}
		})
	}
}

func TestJmpOperandWrapsAroundMemory(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.PC = 0xFFFE
	cpu.Memory[0xFFFE] = r8.OpJmp
	cpu.Memory[0xFFFF] = 0x04 // lo
	cpu.Memory[0x0000] = 0x00 // hi, read after PC wraps
	mustStep(t, cpu)
	if cpu.PC != 4 {
		t.Errorf("want PC == 4, got %d", cpu.PC)
	}
}

func TestBranchWrapsAroundMemory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		op     byte
		a      byte
		pc     uint16
		offset byte
		wantPC uint16
	}{
		{"bra operand read after PC wraps", r8.OpBra, 0, 0xFFFF, 1, 2},
		{"bra target past end of memory", r8.OpBra, 0, 0xFFF0, 0x20, 0x0012},
		{"bra max forward offset", r8.OpBra, 0, 0xFFFE, 0x7F, 0x007F},
		{"bra 0x80 is max backward offset", r8.OpBra, 0, 0x0100, 0x80, 0x0082},
		{"bra target before start of memory", r8.OpBra, 0, 0x0000, 0xFC, 0xFFFE},
		{"beq taken, target before start of memory", r8.OpBeq, 0, 0x0000, 0xFC, 0xFFFE},
		{"bne taken, target before start of memory", r8.OpBne, 1, 0x0000, 0xFC, 0xFFFE},
		{"beq taken, operand read after PC wraps", r8.OpBeq, 0, 0xFFFF, 1, 2},
		{"beq not taken, PC moves past wrapped operand", r8.OpBeq, 1, 0xFFFF, 1, 1},
		{"beq taken, target past end of memory", r8.OpBeq, 0, 0xFFF0, 0x20, 0x0012},
		{"bne taken, operand read after PC wraps", r8.OpBne, 1, 0xFFFF, 1, 2},
		{"bne not taken, PC moves past wrapped operand", r8.OpBne, 0, 0xFFFF, 1, 1},
		{"bne taken, target past end of memory", r8.OpBne, 1, 0xFFF0, 0x20, 0x0012},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := r8.NewCPU()
			cpu.PC = c.pc
			cpu.R[r8.A] = c.a
			cpu.Memory[c.pc] = c.op
			cpu.Memory[c.pc+1] = c.offset
			mustStep(t, cpu)
			if cpu.PC != c.wantPC {
				t.Errorf("want PC == %#04x, got %#04x", c.wantPC, cpu.PC)
			}
		})
	}
}

func TestNextInstruction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pc   uint16
		mem  map[uint16]byte
		want string
	}{
		{"no operand", 0x0000, map[uint16]byte{0x0000: r8.OpIncR}, "inc a"},
		{"unknown opcode", 0x0000, map[uint16]byte{0x0000: 0xFF}, "???"},
		{"ld at last byte", 0xFFFF, map[uint16]byte{0xFFFF: r8.OpLdRN, 0x0000: 5}, "ld a, 0x05"},
		{"jmp at second-last byte", 0xFFFE, map[uint16]byte{0xFFFE: r8.OpJmp, 0xFFFF: 0x34, 0x0000: 0x12}, "jmp 0x1234"},
		{"jmp at last byte", 0xFFFF, map[uint16]byte{0xFFFF: r8.OpJmp, 0x0000: 0x34, 0x0001: 0x12}, "jmp 0x1234"},
		{"bne negative offset", 0x0010, map[uint16]byte{0x0010: r8.OpBne, 0x0011: 0xFD}, "bne 0xFD"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := r8.NewCPU()
			cpu.PC = c.pc
			for addr, b := range c.mem {
				cpu.Memory[addr] = b
			}
			if got := cpu.NextInstruction(); got != c.want {
				t.Errorf("NextInstruction() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestJmpInfiniteLoop(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "LD", "A,", "0x0A", "INC", "A", "JMP", "0x0002")
	for range 1 + 3*2 { // ld, then 3 iterations of inc+jmp
		mustStep(t, cpu)
	}
	if cpu.R[r8.A] != 13 {
		t.Errorf("want A == 13, got %d", cpu.R[r8.A])
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
		{"invalid operand", []string{"LD", "A,", "not_a_number"}, r8.ErrBadOperand},
		{"operand out of range", []string{"LD", "A,", "0x100"}, r8.ErrBadOperand},
		{"unknown instruction", []string{"FOOBAR"}, r8.ErrUnknownInstruction},
		{"missing operand", []string{"LD"}, r8.ErrMissingOperand},
		{"jmp address out of range", []string{"JMP", "0x10000"}, r8.ErrBadOperand},
		{"jmp missing operand", []string{"JMP"}, r8.ErrMissingOperand},
		{"bra missing operand", []string{"BRA"}, r8.ErrMissingOperand},
		{"bra operand out of range", []string{"BRA", "0x100"}, r8.ErrBadOperand},
		{"ld rejects negative operand", []string{"LD", "A,", "-0x01"}, r8.ErrBadOperand},
		{"bra rejects negative operand", []string{"BRA", "-0x03"}, r8.ErrBadOperand},
		{"bra invalid operand", []string{"BRA", "x"}, r8.ErrBadOperand},
		{"beq missing operand", []string{"BEQ"}, r8.ErrMissingOperand},
		{"beq operand out of range", []string{"BEQ", "0x100"}, r8.ErrBadOperand},
		{"bne missing operand", []string{"BNE"}, r8.ErrMissingOperand},
		{"bne operand out of range", []string{"BNE", "0x100"}, r8.ErrBadOperand},
		{"decimal is rejected", []string{"LD", "A,", "5"}, r8.ErrBadOperand},
		{"signed decimal offset is rejected", []string{"BRA", "-3"}, r8.ErrBadOperand},
		{"binary is rejected", []string{"LD", "A,", "0b101"}, r8.ErrBadOperand},
		{"octal is rejected", []string{"LD", "A,", "0o7"}, r8.ErrBadOperand},
		{"uppercase prefix is rejected", []string{"LD", "A,", "0X05"}, r8.ErrBadOperand},
		{"prefix without digits", []string{"LD", "A,", "0x"}, r8.ErrBadOperand},
		{"underscore in digits", []string{"JMP", "0x12_34"}, r8.ErrBadOperand},
		{"too many operands", []string{"LD", "A,", "0x05,", "0x06"}, r8.ErrBadOperand},
		{"too many operands without spaces", []string{"LD", "A,", "0x05,0x06"}, r8.ErrBadOperand},
		{"too many operands with spaced comma", []string{"LD", "A,", "0x05", ",", "0x06"}, r8.ErrBadOperand},
		{"comma after operandless instruction", []string{"INC", "A,", "DEC", "A"}, r8.ErrBadOperand},
		{"comma instead of operand", []string{"LD", ",0x05"}, r8.ErrMissingOperand},
		{"inc missing register", []string{"INC"}, r8.ErrMissingOperand},
		{"dec missing register", []string{"DEC"}, r8.ErrMissingOperand},
		{"ld without register", []string{"LD", "0x05"}, r8.ErrBadRegister},
		{"unknown register", []string{"INC", "Z"}, r8.ErrBadRegister},
		{"number instead of register", []string{"INC", "0x05"}, r8.ErrBadRegister},
		{"ld missing value", []string{"LD", "A"}, r8.ErrMissingOperand},
		{"ld missing value after comma", []string{"LD", "A,"}, r8.ErrMissingOperand},
		{"ld missing comma", []string{"LD", "A", "0x05"}, r8.ErrBadOperand},
		{"inc with extra operand", []string{"INC", "A,", "0x05"}, r8.ErrBadOperand},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := assemble(c.tokens...)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("Assemble(%v): want %v, got %v", c.tokens, c.wantErr, err)
			}
		})
	}
}

func TestAssembleSource(t *testing.T) {
	t.Parallel()
	src := `; count down from 3
ld a, 0x03   ; A = 3
dec a
bne 0xFD  ; back to dec

halt`
	got, err := r8.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{r8.OpLdRN, 3, r8.OpDecR, r8.OpBne, 0xFD, r8.OpHalt}
	if !bytes.Equal(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestAssembleErrorReportsLineNumber(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		src     string
		wantErr error
		wantMsg string
	}{
		{"unknown instruction", "nop\n\nfoo", r8.ErrUnknownInstruction, "line 3:"},
		{"bad operand", "; comment\nld a, 0x100", r8.ErrBadOperand, "line 2:"},
		{"operand must be on the same line", "ld a,\n0x05", r8.ErrMissingOperand, "line 1:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := r8.Assemble(c.src)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
			if !strings.HasPrefix(err.Error(), c.wantMsg) {
				t.Errorf("want error starting %q, got %q", c.wantMsg, err)
			}
		})
	}
}

// nops returns source for n nop instructions, one per line.
func nops(n int) string {
	return strings.Repeat("nop\n", n)
}

func TestAssembleLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want []byte
	}{
		{"backward branch", "ld a, 0x03\nloop: dec a\nbne loop\nhalt", []byte{r8.OpLdRN, 3, r8.OpDecR, r8.OpBne, 0xFD, r8.OpHalt}},
		{"forward branch", "beq done\ninc a\ndone: halt", []byte{r8.OpBeq, 1, r8.OpIncR, r8.OpHalt}},
		{"jmp to absolute address", "jmp start\ninc a\nstart: halt", []byte{r8.OpJmp, 0x04, 0x00, r8.OpIncR, r8.OpHalt}},
		{"label on its own line", "loop:\ndec a\nbne loop", []byte{r8.OpDecR, r8.OpBne, 0xFD}},
		{"two labels at one address", "a:\nb: inc a\nbra a\nbra b", []byte{r8.OpIncR, r8.OpBra, 0xFD, r8.OpBra, 0xFB}},
		{"label at end of program", "bra end\nend:", []byte{r8.OpBra, 0}},
		{"label named like a mnemonic", "inc: bra inc", []byte{r8.OpBra, 0xFE}},
		{"numbers still work beside labels", "x: bra 0xFE\njmp x", []byte{r8.OpBra, 0xFE, r8.OpJmp, 0, 0}},
		{
			"max forward offset",
			"bra far\n" + nops(127) + "far: halt",
			slices.Concat([]byte{r8.OpBra, 0x7F}, bytes.Repeat([]byte{r8.OpNop}, 127), []byte{r8.OpHalt}),
		},
		{
			"max backward offset",
			"back:\n" + nops(126) + "bra back",
			slices.Concat(bytes.Repeat([]byte{r8.OpNop}, 126), []byte{r8.OpBra, 0x80}),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := r8.Assemble(c.src)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, c.want) {
				t.Errorf("Assemble(%q) = % X, want % X", c.src, got, c.want)
			}
		})
	}
}

func TestAssembleLabelErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		src     string
		wantErr error
		wantMsg string
	}{
		{"undefined label", "nop\nbra nowhere", r8.ErrUndefinedLabel, "line 2:"},
		{"undefined label is a bad operand", "nop\nbra nowhere", r8.ErrBadOperand, "line 2:"},
		{"labels are case-sensitive", "Loop: bra loop", r8.ErrUndefinedLabel, "line 1:"},
		{"duplicate label", "a: nop\na: halt", r8.ErrDuplicateLabel, "line 2:"},
		{"name starts with digit", "1abc: nop", r8.ErrBadLabel, "line 1:"},
		{"empty name", ": nop", r8.ErrBadLabel, "line 1:"},
		{"forward branch out of range", "bra far\n" + nops(128) + "far:", r8.ErrBadOperand, "line 1:"},
		{"backward branch out of range", "back:\n" + nops(127) + "bra back", r8.ErrBadOperand, "line 129:"},
		{"ld rejects a label", "x: ld a, x", r8.ErrBadOperand, "line 1:"},
		{"program overlaps reset vector", nops(int(r8.ResetVector) + 1), r8.ErrProgramTooLarge, "line 65535:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := r8.Assemble(c.src)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
			if !strings.HasPrefix(err.Error(), c.wantMsg) {
				t.Errorf("want error starting %q, got %q", c.wantMsg, err)
			}
		})
	}
}

func TestAssembleLabelsRun(t *testing.T) {
	t.Parallel()
	program, err := r8.Assemble(`
		ld a, 0x05
	loop:
		dec a
		beq done
		jmp loop
	done:
		halt`)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	if err := cpu.LoadProgram(program, 0); err != nil {
		t.Fatal(err)
	}
	if err := cpu.RunN(100); err != nil {
		t.Fatal(err)
	}
	if !cpu.Halted || cpu.R[r8.A] != 0 {
		t.Errorf("want Halted with A == 0, got Halted=%v A=%d", cpu.Halted, cpu.R[r8.A])
	}
}

func TestAssembleDisassemble(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		inTokens []string
		wantAsm  []byte
		wantDasm string
	}{
		{"empty", []string{}, nil, ""},
		{"nop", []string{"NOP"}, []byte{0x01}, "nop"},
		{"inc a", []string{"INC", "A"}, []byte{0x30}, "inc a"},
		{"dec a", []string{"DEC", "A"}, []byte{0x40}, "dec a"},
		{"halt", []string{"HALT"}, []byte{0x00}, "halt"},
		{"ld a, 0x14", []string{"LD", "A,", "0x14"}, []byte{0x10, 0x14}, "ld a, 0x14"},
		{"lowercase hex digits", []string{"LD", "A,", "0xff"}, []byte{0x10, 0xFF}, "ld a, 0xFF"},
		{"comma without spaces", []string{"LD", "A,0x14"}, []byte{0x10, 0x14}, "ld a, 0x14"},
		{"comma before space", []string{"LD", "A", ",0x14"}, []byte{0x10, 0x14}, "ld a, 0x14"},
		{"lowercase register", []string{"ld", "a,", "0x14"}, []byte{0x10, 0x14}, "ld a, 0x14"},
		{"mixedcase", []string{"nop", "Inc", "a", "dEc", "A", "HaLt"}, []byte{0x01, 0x30, 0x40, 0x00}, "nop\ninc a\ndec a\nhalt"},
		{"sequence", []string{"INC", "A", "INC", "A", "DEC", "A", "HALT"}, []byte{0x30, 0x30, 0x40, 0x00}, "inc a\ninc a\ndec a\nhalt"},
		{"jmp 0x0002", []string{"JMP", "0x0002"}, []byte{0xF7, 0x02, 0x00}, "jmp 0x0002"},
		{"jmp high byte", []string{"JMP", "0x1234"}, []byte{0xF7, 0x34, 0x12}, "jmp 0x1234"},
		{"jmp short hex in, four digits out", []string{"JMP", "0x100"}, []byte{0xF7, 0x00, 0x01}, "jmp 0x0100"},
		{"bra 0x03", []string{"BRA", "0x03"}, []byte{0xF0, 0x03}, "bra 0x03 ; bra 3"},
		{"bra 0x00", []string{"BRA", "0x00"}, []byte{0xF0, 0x00}, "bra 0x00 ; bra 0"},
		{"bra max forward", []string{"BRA", "0x7F"}, []byte{0xF0, 0x7F}, "bra 0x7F ; bra 127"},
		{"bra 0xFF is -1", []string{"BRA", "0xFF"}, []byte{0xF0, 0xFF}, "bra 0xFF ; bra -1"},
		{"bra 0x80 is -128", []string{"BRA", "0x80"}, []byte{0xF0, 0x80}, "bra 0x80 ; bra -128"},
		{"beq 0xFD", []string{"BEQ", "0xFD"}, []byte{0xF1, 0xFD}, "beq 0xFD ; beq -3"},
		{"bne 0xFD", []string{"BNE", "0xFD"}, []byte{0xF2, 0xFD}, "bne 0xFD ; bne -3"},
		{"bne backward loop", []string{"LD", "A,", "0x03", "DEC", "A", "BNE", "0xFD", "HALT"}, []byte{0x10, 0x03, 0x40, 0xF2, 0xFD, 0x00}, "ld a, 0x03\ndec a\nbne 0xFD ; bne -3\nhalt"},
		{"bra in sequence", []string{"BRA", "0x01", "INC", "A", "HALT"}, []byte{0xF0, 0x01, 0x30, 0x00}, "bra 0x01 ; bra 1\ninc a\nhalt"},
		{"beq in sequence", []string{"LD", "A,", "0x00", "BEQ", "0x01", "INC", "A", "HALT"}, []byte{0x10, 0x00, 0xF1, 0x01, 0x30, 0x00}, "ld a, 0x00\nbeq 0x01 ; beq 1\ninc a\nhalt"},
		{"bne in sequence", []string{"LD", "A,", "0x05", "BNE", "0x01", "INC", "A", "HALT"}, []byte{0x10, 0x05, 0xF2, 0x01, 0x30, 0x00}, "ld a, 0x05\nbne 0x01 ; bne 1\ninc a\nhalt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			asm, err := assemble(c.inTokens...)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(asm, c.wantAsm) {
				t.Errorf("Assemble(%v)=%v want %v", c.inTokens, asm, c.wantAsm)
			}

			dasm := r8.Disassemble(asm)
			if dasm != c.wantDasm {
				t.Errorf("roundtrip %v -> dasm=%#v want %#v", c.inTokens, dasm, c.wantDasm)
			}

			reasm, err := r8.Assemble(dasm)
			if err != nil {
				t.Fatalf("Assemble(Disassemble(...)): %v", err)
			}
			if !bytes.Equal(reasm, asm) {
				t.Errorf("Assemble(%q)=% X want % X", dasm, reasm, asm)
			}
		})
	}
}
