// Package r8 emulates a simple CPU called the R8.
package r8

import (
	"encoding/binary"
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
	mnemonic    string
	operandSize int // width in bytes of the operand after the op code (0, 1 or 2), little-endian
	exec        func(cpu *CPU) bool
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
	OpJmp: {"jmp", 2, func(cpu *CPU) bool {
		cpu.PC = cpu.readUint16(cpu.PC)
		return true
	}},
	OpBra: {"bra", 1, func(cpu *CPU) bool {
		cpu.PC += uint16(cpu.Mem[cpu.PC]) + 1
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

// readUint16 returns the little-endian 16-bit value at addr.
// The high byte address wraps at 0xFFFF like real 16-bit hardware.
func (cpu *CPU) readUint16(addr uint16) uint16 {
	lo := cpu.Mem[addr]
	hi := cpu.Mem[addr+1]
	return uint16(hi)<<8 | uint16(lo)
}

// String returns a one-line view of the next instruction, PC, and A, as
// shown by the monitor.
func (cpu *CPU) String() string {
	return fmt.Sprintf("next %-6q => %04d > %07d", strings.ToUpper(disassembleOp(*cpu)), cpu.PC, cpu.A)
}

// disassembleOp returns the source form of the instruction at cpu.PC, such
// as "ld 5" or "halt", for the monitor's status line. It returns "???" if the
// opcode is unknown.
func disassembleOp(cpu CPU) string {
	b := cpu.Mem[cpu.PC]
	inst, ok := instructions[b]
	if !ok {
		return "???"

	}
	if inst.operandSize == 0 {
		return inst.mnemonic
	}
	return Disassemble(cpu.Mem[cpu.PC : cpu.PC+uint16(inst.operandSize+1)])
}

// Disassemble converts machine code into one line of source per instruction.
// Byte operands are shown in decimal and 16-bit addresses in hex. Unknown
// opcodes and a final instruction missing its operands are shown as "???".
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
		lines = append(lines, inst.mnemonic+" "+formatOperand(data[i:i+inst.operandSize]))
		i += inst.operandSize
	}
	return strings.Join(lines, "\n")
}

func formatOperand(b []byte) string {
	if len(b) == 2 {
		return fmt.Sprintf("0x%04X", binary.LittleEndian.Uint16(b))
	}
	return strconv.Itoa(int(b[0]))
}

// Assemble converts source tokens into machine code. Mnemonics are
// case-insensitive, and operands may be written in any base accepted by
// strconv.ParseUint with base 0. Multi-byte operands are stored
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

		size := instructions[op].operandSize
		if size == 0 {
			continue
		}
		i++
		if i == len(tokens) {
			return []byte{}, fmt.Errorf("%s: %w", tok, ErrMissingOperand)
		}
		v, err := strconv.ParseUint(tokens[i], 0, 8*size)
		if err != nil {
			return []byte{}, fmt.Errorf("%s: %w %q: %w", tok, ErrBadOperand, tokens[i], err)
		}
		program = appendOperand(program, v, size)
	}
	return program, nil
}

func appendOperand(program []byte, v uint64, size int) []byte {
	if size == 2 {
		return binary.LittleEndian.AppendUint16(program, uint16(v))
	}
	return append(program, byte(v))
}
