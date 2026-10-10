// Package r8 emulates a simple CPU called the R8.
package r8

import (
	"errors"
	"fmt"
	"os"
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
	// Mem is the CPU's addressable memory.
	Mem [MemSize]byte
}

type instruction struct {
	mnemonic string
	operands int // number of operand bytes following the op code
	exec     func(cpu *CPU) bool
}

// Opcodes of the R8 instruction set.
const (
	OpHalt byte = 0x00 // stop execution
	OpNop  byte = 0x01 // do nothing
	OpInc  byte = 0x30 // increment A
	OpDec  byte = 0x40 // decrement A
	OpLd   byte = 0x10 // load the following byte into A
)

// MemSize is the size of the CPU's memory in bytes.
const MemSize = 65536

// Errors returned by LoadProgram, LoadFile, and Assemble. Use errors.Is to
// check for them, since they are usually wrapped with more context.
var (
	ErrProgramTooLarge    = errors.New("program does not fit into memory")
	ErrUnknownInstruction = errors.New("unknown instruction")
	ErrBadOperand         = errors.New("bad operand")
	ErrMissingOperand     = errors.New("missing operand")
)

var instructions = map[byte]instruction{
	OpHalt: {"halt", 0, func(cpu *CPU) bool {
		return false
	}},
	OpNop: {"nop", 0, func(cpu *CPU) bool {
		return true
	}},
	OpInc: {"inc", 0, func(cpu *CPU) bool {
		cpu.A++
		return true
	}},
	OpDec: {"dec", 0, func(cpu *CPU) bool {
		cpu.A--
		return true
	}},
	OpLd: {"ld", 1, func(cpu *CPU) bool {
		cpu.A = cpu.Mem[cpu.PC]
		cpu.PC++
		return true
	}},
}

var opcodes = func() map[string]byte {
	m := make(map[string]byte, len(instructions))
	for op, inst := range instructions {
		m[inst.mnemonic] = op
	}
	return m
}()

// NewCPU returns a CPU in its initial state.
func NewCPU() *CPU {
	return &CPU{}
}

// Step executes the instruction at PC and reports whether the CPU should
// keep running. It returns false on HALT or an unknown opcode.
func (cpu *CPU) Step() bool {
	opcode := cpu.Mem[cpu.PC]
	cpu.PC++
	inst, ok := instructions[opcode]
	if !ok {
		return false
	}
	return inst.exec(cpu)
}

// Run executes instructions until Step returns false.
func (cpu *CPU) Run() {
	for cpu.Step() {
	}
}

// LoadFile reads the program in the file at path and loads it into memory
// starting at address 0.
func (cpu *CPU) LoadFile(path string) error {
	program, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := cpu.LoadProgram(program); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	return nil
}

// LoadProgram copies program into memory starting at address 0. It returns
// ErrProgramTooLarge if program is larger than MemSize.
func (cpu *CPU) LoadProgram(program []byte) error {
	if len(program) > MemSize {
		return fmt.Errorf("%w: %d bytes, max %d", ErrProgramTooLarge, len(program), MemSize)
	}
	copy(cpu.Mem[0:], program)
	return nil
}

// String returns a one-line view of the next instruction, PC, and A, as
// shown by the monitor.
func (cpu *CPU) String() string {
	return fmt.Sprintf("next %-6q => %04d > %07d", strings.ToUpper(byteToString(cpu.Mem[cpu.PC])), cpu.PC, cpu.A)
}

func byteToString(b byte) string {
	if inst, ok := instructions[b]; ok {
		return inst.mnemonic
	}
	return "???"
}

// Disassemble converts machine code into one line of source per instruction,
// with operands in decimal. Unknown opcodes and a final instruction missing
// its operands are shown as "???".
func Disassemble(data []byte) []string {
	lines := make([]string, 0)
	pending := 0

	for _, b := range data {
		if pending > 0 {
			lines[len(lines)-1] += fmt.Sprintf(" %d", b)
			pending--
			continue
		}

		inst, ok := instructions[b]
		if !ok {
			lines = append(lines, "???")
			continue
		}

		lines = append(lines, inst.mnemonic)
		pending = inst.operands
	}

	if pending > 0 {
		lines = append(lines, "???")
	}
	return lines
}

// Assemble converts source tokens into machine code. Mnemonics are
// case-insensitive, and operands may be written in any base accepted by
// strconv.ParseUint with base 0. Errors wrap ErrUnknownInstruction,
// ErrBadOperand, or ErrMissingOperand.
func Assemble(tokens []string) ([]byte, error) {
	program := make([]byte, 0)
	var current string // mnemonic waiting for operands
	pending := 0

	for _, tok := range tokens {
		if pending > 0 {
			v, err := strconv.ParseUint(tok, 0, 8)
			if err != nil {
				return []byte{}, fmt.Errorf("%s: %w %q: %w", current, ErrBadOperand, tok, err)
			}
			program = append(program, byte(v))
			pending--
			continue
		}

		op, ok := opcodes[strings.ToLower(tok)]
		if !ok {
			return []byte{}, fmt.Errorf("%w %q", ErrUnknownInstruction, tok)
		}
		program = append(program, op)
		current = tok
		pending = instructions[op].operands
	}

	if pending > 0 {
		return []byte{}, fmt.Errorf("%s: %w", current, ErrMissingOperand)
	}

	return program, nil
}
