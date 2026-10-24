package r8_test

import (
	"bytes"
	"errors"
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
	if err := cpu.LoadProgram(program, 0); err != nil {
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
	got := cpu.Memory[0]
	if got != 0 {
		t.Errorf("after New, want Memory[0] == 0, got %d", got)
	}

	if cpu.A != 0 {
		t.Errorf("after New, want a == 0, got %d", cpu.A)
	}
	if cpu.Halted {
		t.Error("after New, want Halted == false, got true")
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

func TestStepReturnsIllegalInstructionErrorOnUnknownOpcode(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Memory[0] = 0xFF // not in instructions table

	err := cpu.Step()
	var illegal *r8.IllegalInstructionError
	if !errors.As(err, &illegal) {
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
	cpu := newTestCPU(t, "INC")
	cpu.Halted = true
	if err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if cpu.PC != 0 {
		t.Errorf("want PC == 0, got %d", cpu.PC)
	}
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
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
	cpu := newTestCPU(t, "INC", "JMP", "0x0000")
	err := cpu.RunN(10)
	if !errors.Is(err, r8.ErrStepLimit) {
		t.Fatalf("want ErrStepLimit, got %v", err)
	}
	if cpu.A != 5 {
		t.Errorf("want A == 5 after 10 steps, got %d", cpu.A)
	}
}

func TestRunNStopsAtHalt(t *testing.T) {
	t.Parallel()
	cpu := newTestCPU(t, "INC", "HALT", "INC")
	if err := cpu.RunN(10); err != nil {
		t.Fatal(err)
	}
	if !cpu.Halted || cpu.A != 1 {
		t.Errorf("want Halted with A == 1, got Halted=%v A=%d", cpu.Halted, cpu.A)
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
		{"beq taken when A is zero", []string{"BEQ", "1", "INC", "HALT"}, 0, 0, 0, 4},
		{"beq not taken when A is nonzero", []string{"LD", "5", "BEQ", "1", "INC", "HALT"}, 0, 0, 6, 6},
		{"beq taken after dec reaches zero", []string{"LD", "1", "DEC", "BEQ", "1", "INC", "HALT"}, 0, 0, 0, 7},
		{"beq 0 falls through", []string{"BEQ", "0", "INC", "HALT"}, 0, 0, 1, 4},
		{"bne taken when A is nonzero", []string{"LD", "5", "BNE", "1", "INC", "HALT"}, 0, 0, 5, 6},
		{"bne not taken when A is zero", []string{"BNE", "1", "INC", "HALT"}, 0, 0, 1, 4},
		{"bne not taken after dec reaches zero", []string{"LD", "1", "DEC", "BNE", "1", "INC", "HALT"}, 0, 0, 1, 7},
		{"bne 0 falls through", []string{"LD", "5", "BNE", "0", "INC", "HALT"}, 0, 0, 6, 6},
		{"bne backward loops until A is zero", []string{"LD", "3", "DEC", "BNE", "-3", "HALT"}, 0, 0, 0, 6},
		{"beq backward taken once", []string{"BRA", "1", "HALT", "LD", "0", "BEQ", "-5"}, 0, 0, 0, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := newTestCPU(t, c.tokens...)
			cpu.A = c.initA
			cpu.PC = c.initPC
			if err := cpu.Run(); err != nil {
				t.Fatal(err)
			}
			if !cpu.Halted {
				t.Error("want Halted == true after Run, got false")
			}
			if cpu.A != c.wantA {
				t.Errorf("want A == %d, got %d", c.wantA, cpu.A)
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
		{"one byte past end of memory", 2, 0xFFFF},
		{"past end of memory at offset", 0x11, 0xFFF0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cpu := r8.NewCPU()
			err := cpu.LoadProgram(make([]byte, c.size), c.addr)
			if !errors.Is(err, r8.ErrProgramTooLarge) {
				t.Errorf("want ErrProgramTooLarge, got %v", err)
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
		{"whole memory at 0", r8.MemSize, 0},
		{"last byte of memory", 1, 0xFFFF},
		{"up to end of memory at offset", 0x10, 0xFFF0},
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
	program := []byte{r8.OpLd, 7, r8.OpHalt}
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
	program := []byte{r8.OpLd, 7, r8.OpHalt}
	if err := cpu.LoadProgram(program, 0x0100); err != nil {
		t.Fatal(err)
	}
	if cpu.PC != 0x0100 {
		t.Errorf("want PC 0x0100 after load, got %#04x", cpu.PC)
	}
	if err := cpu.Run(); err != nil {
		t.Fatal(err)
	}
	if cpu.A != 7 {
		t.Errorf("want A 7, got %d", cpu.A)
	}
	if cpu.PC != 0x0103 {
		t.Errorf("want PC 0x0103, got %#04x", cpu.PC)
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
		{"ld missing operand", []byte{r8.OpLd}, "ld\n???"},
		{"jmp missing operand byte", []byte{r8.OpJmp, 0x02}, "jmp\n???"},
		{"bra missing operand", []byte{r8.OpBra}, "bra\n???"},
		{"beq missing operand", []byte{r8.OpBeq}, "beq\n???"},
		{"bne missing operand", []byte{r8.OpBne}, "bne\n???"},
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
	cpu.Step()
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
			cpu.A = c.a
			cpu.Memory[c.pc] = c.op
			cpu.Memory[c.pc+1] = c.offset
			cpu.Step()
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
		{"no operand", 0x0000, map[uint16]byte{0x0000: r8.OpInc}, "inc"},
		{"unknown opcode", 0x0000, map[uint16]byte{0x0000: 0xFF}, "???"},
		{"ld at last byte", 0xFFFF, map[uint16]byte{0xFFFF: r8.OpLd, 0x0000: 5}, "ld 5"},
		{"jmp at second-last byte", 0xFFFE, map[uint16]byte{0xFFFE: r8.OpJmp, 0xFFFF: 0x34, 0x0000: 0x12}, "jmp 0x1234"},
		{"jmp at last byte", 0xFFFF, map[uint16]byte{0xFFFF: r8.OpJmp, 0x0000: 0x34, 0x0001: 0x12}, "jmp 0x1234"},
		{"bne negative offset", 0x0010, map[uint16]byte{0x0010: r8.OpBne, 0x0011: 0xFD}, "bne -3"},
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
		{"bra operand above signed range", []string{"BRA", "128"}, r8.ErrBadOperand},
		{"bra operand below signed range", []string{"BRA", "-129"}, r8.ErrBadOperand},
		{"bra unsigned hex is out of range", []string{"BRA", "0xFF"}, r8.ErrBadOperand},
		{"ld rejects negative operand", []string{"LD", "-1"}, r8.ErrBadOperand},
		{"jmp rejects negative operand", []string{"JMP", "-1"}, r8.ErrBadOperand},
		{"bra invalid operand", []string{"BRA", "x"}, r8.ErrBadOperand},
		{"beq missing operand", []string{"BEQ"}, r8.ErrMissingOperand},
		{"beq operand out of range", []string{"BEQ", "128"}, r8.ErrBadOperand},
		{"bne missing operand", []string{"BNE"}, r8.ErrMissingOperand},
		{"bne operand out of range", []string{"BNE", "-129"}, r8.ErrBadOperand},
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
		{"bra hex in, decimal out", []string{"BRA", "0x7F"}, []byte{0xF0, 0x7F}, "bra 127", false},
		{"bra -1", []string{"BRA", "-1"}, []byte{0xF0, 0xFF}, "bra -1", false},
		{"bra -128", []string{"BRA", "-128"}, []byte{0xF0, 0x80}, "bra -128", false},
		{"beq -3", []string{"BEQ", "-3"}, []byte{0xF1, 0xFD}, "beq -3", false},
		{"bne -3", []string{"BNE", "-3"}, []byte{0xF2, 0xFD}, "bne -3", false},
		{"bne backward loop", []string{"LD", "3", "DEC", "BNE", "-3", "HALT"}, []byte{0x10, 0x03, 0x40, 0xF2, 0xFD, 0x00}, "ld 3\ndec\nbne -3\nhalt", false},
		{"bra in sequence", []string{"BRA", "1", "INC", "HALT"}, []byte{0xF0, 0x01, 0x30, 0x00}, "bra 1\ninc\nhalt", false},
		{"beq 3", []string{"BEQ", "3"}, []byte{0xF1, 0x03}, "beq 3", false},
		{"beq in sequence", []string{"LD", "0", "BEQ", "1", "INC", "HALT"}, []byte{0x10, 0x00, 0xF1, 0x01, 0x30, 0x00}, "ld 0\nbeq 1\ninc\nhalt", false},
		{"bne 3", []string{"BNE", "3"}, []byte{0xF2, 0x03}, "bne 3", false},
		{"bne in sequence", []string{"LD", "5", "BNE", "1", "INC", "HALT"}, []byte{0x10, 0x05, 0xF2, 0x01, 0x30, 0x00}, "ld 5\nbne 1\ninc\nhalt", false},
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
