// Package r8 emulates a simple CPU called the R8.
package r8

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CPU is an R8 CPU clocked at 4MHz, with 64KiB of static RAM, an 8-bit data
// bus, and a 16-bit address bus. The zero value is a CPU in its initial state.
//
// See https://github.com/bitfield/r8/blob/main/crates/rx82/README.md#the-rx82-architecture
type CPU struct {
	// PC is the program counter: the address of the next byte to execute.
	PC uint16
	// A is the accumulator register.
	A byte
	// Memory is the 64KB CPU's addressable memory.
	Memory [MemSize]byte
	// Halted is true once a HALT instruction has executed.
	Halted bool
}

type instruction struct {
	mnemonic    string
	operandSize int  // width in bytes of the operand after the op code (0, 1 or 2), little-endian
	signed      bool // operand is a two's complement offset rather than an unsigned value
	handler     func(*CPU) error
}

// Opcodes of the R8 instruction set.
const (
	OpHalt byte = 0x00 // stop execution
	OpNop  byte = 0x01 // do nothing
	OpInc  byte = 0x30 // increment A
	OpDec  byte = 0x40 // decrement A
	OpLd   byte = 0x10 // load the following byte into A
	OpJmp  byte = 0xF7 // jump to the following 16-bit little-endian address
	OpBra  byte = 0xF0 // branch by the following signed byte, relative to the next instruction
	OpBeq  byte = 0xF1 // branch by the following signed byte if A is zero
	OpBne  byte = 0xF2 // branch by the following signed byte if A is not zero
)

// MemSize is the size of the CPU's memory in bytes.
const MemSize = 65536

// Errors returned by LoadProgram and Assemble. Use errors.Is to
// check for them, since they are usually wrapped with more context.
var (
	ErrProgramTooLarge    = errors.New("program does not fit into memory")
	ErrUnknownInstruction = errors.New("unknown instruction")
	ErrBadOperand         = errors.New("bad operand")
	ErrMissingOperand     = errors.New("missing operand")
)

// IllegalInstructionError reports an undefined opcode and the address it was
// fetched from.
type IllegalInstructionError struct {
	Opcode byte
	PC     uint16
}

func (e *IllegalInstructionError) Error() string {
	return fmt.Sprintf("illegal instruction: 0x%02X at PC=0x%04X", e.Opcode, e.PC)
}

var instructions = map[byte]instruction{
	OpHalt: {mnemonic: "halt", handler: func(cpu *CPU) error {
		cpu.Halted = true
		cpu.PC += 1
		return nil
	}},
	OpNop: {mnemonic: "nop", handler: func(cpu *CPU) error {
		cpu.PC += 1
		return nil
	}},
	OpInc: {mnemonic: "inc", handler: func(cpu *CPU) error {
		cpu.A++
		cpu.PC += 1
		return nil
	}},
	OpDec: {mnemonic: "dec", handler: func(cpu *CPU) error {
		cpu.A--
		cpu.PC += 1
		return nil
	}},
	OpLd: {mnemonic: "ld", operandSize: 1, handler: func(cpu *CPU) error {
		cpu.A = cpu.Memory[cpu.PC+1]
		cpu.PC += 2
		return nil
	}},
	OpJmp: {mnemonic: "jmp", operandSize: 2, handler: func(cpu *CPU) error {
		cpu.PC = cpu.readUint16(cpu.PC + 1)
		return nil
	}},
	OpBra: {mnemonic: "bra", operandSize: 1, signed: true, handler: func(cpu *CPU) error {
		cpu.branch(true)
		return nil
	}},
	OpBeq: {mnemonic: "beq", operandSize: 1, signed: true, handler: func(cpu *CPU) error {
		cpu.branch(cpu.A == 0)
		return nil
	}},
	OpBne: {mnemonic: "bne", operandSize: 1, signed: true, handler: func(cpu *CPU) error {
		cpu.branch(cpu.A != 0)
		return nil
	}},
}

var opcodes = func() map[string]byte {
	m := make(map[string]byte, len(instructions))
	for op, inst := range instructions {
		m[inst.mnemonic] = op
	}
	return m
}()

// branch moves PC past the offset operand and then, if taken, by that
// signed offset (-128 to 127). The target wraps around memory in either
// direction.
func (cpu *CPU) branch(taken bool) {
	offset := int8(cpu.Memory[cpu.PC+1])
	cpu.PC += 2
	if taken {
		cpu.PC += uint16(offset)
	}
}

// NewCPU returns a CPU in its initial state.
func NewCPU() *CPU {
	return &CPU{
		A:      0,
		PC:     0,
		Memory: [MemSize]byte{},
		Halted: false,
	}
}

// Step executes the instruction at PC. It does nothing once the CPU is
// halted. On an unknown opcode it moves PC past it and returns an
// *IllegalInstructionError.
func (cpu *CPU) Step() error {
	if cpu.Halted {
		return nil // Already halted, nothing to do
	}

	opcode := cpu.Memory[cpu.PC]
	inst, ok := instructions[opcode]
	if !ok {
		err := &IllegalInstructionError{
			Opcode: opcode,
			PC:     cpu.PC,
		}
		cpu.PC++
		return err
	}
	return inst.handler(cpu)
}

// Run executes instructions until the CPU halts or Step returns an error.
func (cpu *CPU) Run() error {
	for !cpu.Halted {
		if err := cpu.Step(); err != nil {
			return err
		}
	}
	return nil
}

// LoadProgram copies program into memory starting at addr and sets PC to
// addr. It returns ErrProgramTooLarge, leaving the CPU unchanged, if program
// does not fit between addr and the end of memory.
func (cpu *CPU) LoadProgram(program []byte, addr uint16) error {
	availableSpace := MemSize - int(addr)
	if len(program) > availableSpace {
		return fmt.Errorf("%w: %d bytes at 0x%04X exceeds memory (available: %d bytes)",
			ErrProgramTooLarge, len(program), addr, availableSpace)
	}
	copy(cpu.Memory[addr:], program)
	cpu.PC = addr
	return nil
}

// readUint16 returns the little-endian 16-bit value at addr.
// The high byte address wraps at 0xFFFF like real 16-bit hardware.
func (cpu *CPU) readUint16(addr uint16) uint16 {
	lo := cpu.Memory[addr]
	hi := cpu.Memory[addr+1]
	return uint16(hi)<<8 | uint16(lo)
}

// String returns a one-line view of the next instruction, PC, and A, as
// shown by the monitor.
func (cpu *CPU) String() string {
	return fmt.Sprintf("next %-12q => %04d > %07d", strings.ToUpper(disassembleOp(cpu)), cpu.PC, cpu.A)
}

// disassembleOp returns the source form of the instruction at cpu.PC, such
// as "ld 5" or "halt", for the monitor's status line. It returns "???" if the
// opcode is unknown. Operand bytes wrap around the end of memory, as in Step.
func disassembleOp(cpu *CPU) string {
	code := []byte{cpu.Memory[cpu.PC]}
	for i := range instructions[code[0]].operandSize {
		code = append(code, cpu.Memory[cpu.PC+uint16(i+1)])
	}
	return Disassemble(code)
}

// Disassemble converts machine code into one line of source per instruction.
// Byte operands are shown in decimal (signed for branch offsets) and 16-bit
// addresses in hex. Unknown opcodes and a final instruction missing its
// operands are shown as "???".
func Disassemble(data []byte) string {
	lines := make([]string, 0)
	for i := 0; i < len(data); {
		inst, ok := instructions[data[i]]
		i++
		if !ok {
			lines = append(lines, "???")
			continue
		}
		if inst.operandSize == 0 {
			lines = append(lines, inst.mnemonic)
			continue
		}
		if i+inst.operandSize > len(data) {
			lines = append(lines, inst.mnemonic, "???")
			break
		}
		lines = append(lines, inst.mnemonic+" "+formatOperand(data[i:i+inst.operandSize], inst.signed))
		i += inst.operandSize
	}
	return strings.Join(lines, "\n")
}

func formatOperand(b []byte, signed bool) string {
	if len(b) == 2 {
		return fmt.Sprintf("0x%04X", binary.LittleEndian.Uint16(b))
	}
	if signed {
		return strconv.Itoa(int(int8(b[0])))
	}
	return strconv.Itoa(int(b[0]))
}

// Assemble converts source tokens into machine code. Mnemonics are
// case-insensitive, and operands may be written in any base accepted by
// strconv.ParseUint with base 0. Branch offsets are signed (-128 to 127) and
// parsed with strconv.ParseInt instead. Multi-byte operands are stored
// little-endian. Errors wrap ErrUnknownInstruction, ErrBadOperand, or
// ErrMissingOperand.
func Assemble(tokens []string) ([]byte, error) {
	program := make([]byte, 0)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		op, ok := opcodes[strings.ToLower(tok)]
		if !ok {
			return []byte{}, fmt.Errorf("%w %q", ErrUnknownInstruction, tok)
		}
		program = append(program, op)

		inst := instructions[op]
		if inst.operandSize == 0 {
			continue
		}
		i++
		if i == len(tokens) {
			return []byte{}, fmt.Errorf("%s: %w", tok, ErrMissingOperand)
		}
		v, err := parseOperand(tokens[i], inst.operandSize, inst.signed)
		if err != nil {
			return []byte{}, fmt.Errorf("%s: %w %q: %w", tok, ErrBadOperand, tokens[i], err)
		}
		program = appendOperand(program, v, inst.operandSize)
	}
	return program, nil
}

// parseOperand parses s as a size-byte operand and returns its bit pattern.
// A signed operand is stored in two's complement.
func parseOperand(s string, size int, signed bool) (uint64, error) {
	if !signed {
		return strconv.ParseUint(s, 0, 8*size)
	}
	v, err := strconv.ParseInt(s, 0, 8*size)
	return uint64(v), err
}

func appendOperand(program []byte, v uint64, size int) []byte {
	if size == 2 {
		return binary.LittleEndian.AppendUint16(program, uint16(v))
	}
	return append(program, byte(v))
}
