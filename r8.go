// Package r8 emulates a simple CPU called the R8.
package r8

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// https://github.com/bitfield/r8/blob/main/crates/rx82/README.md#the-rx82-architecture
// R8 CPU clocked at 4Mhz, 64KiB of static RAM, an 8-bit data bus, and a 16-bit address bus.
type CPU struct {
	PC  uint16
	A   byte
	Mem [MEMSIZE]byte
}

type instruction struct {
	mnemonic string
	operands int // number of operand bytes following the op code
	exec     func(cpu *CPU) bool
}

const (
	HALT byte = 0x00
	NOP  byte = 0x01
	INC  byte = 0x30
	DEC  byte = 0x40

	MEMSIZE = 65536
)

var instructions = map[byte]instruction{
	HALT: {"halt", 0, func(cpu *CPU) bool { return false }},
	NOP:  {"nop", 0, func(cpu *CPU) bool { return true }},
	INC:  {"inc", 0, func(cpu *CPU) bool { cpu.A++; return true }},
	DEC:  {"dec", 0, func(cpu *CPU) bool { cpu.A--; return true }},
}

var opcodes = func() map[string]byte {
	m := make(map[string]byte, len(instructions))
	for op, inst := range instructions {
		m[inst.mnemonic] = op
	}
	return m
}()

func NewCPU() *CPU {
	return &CPU{}
}

func (cpu *CPU) Step() bool {
	opcode := cpu.Mem[cpu.PC]
	cpu.PC++
	intst, ok := instructions[opcode]
	if !ok {
		return false
	}
	return intst.exec(cpu)
}

func (cpu *CPU) Run() {
	for cpu.Step() {
	}
}

func (cpu *CPU) LoadFile(path string) {
	program, err := os.ReadFile(path)
	if err != nil {
		return
	}
	err = cpu.LoadProgram(program)
	if err != nil {
		return
	}
}

func (cpu *CPU) LoadProgram(program []byte) error {
	if len(program) > MEMSIZE {
		return errors.New("program does not fit into the memory")
	}
	copy(cpu.Mem[0:], program)
	return nil
}

func (cpu *CPU) String() string {
	return fmt.Sprintf("next %-6q => %04d > %07d", strings.ToUpper(byteToString(cpu.Mem[cpu.PC])), cpu.PC, cpu.A)
}

func byteToString(b byte) string {
	if inst, ok := instructions[b]; ok {
		return inst.mnemonic
	}
	return "unknown"
}

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
			lines = append(lines, "unknown")
			continue
		}

		lines = append(lines, inst.mnemonic)
		pending = inst.operands
	}
	return lines
}

func Assemble(tokens []string) ([]byte, error) {
	program := make([]byte, 0)
	var current string // mnemonic waiting for operands
	pending := 0

	for _, tok := range tokens {
		if pending > 0 {
			v, err := strconv.ParseUint(tok, 0, 8)
			if err != nil {
				return []byte{}, fmt.Errorf("%s: bad operand %q: %w", current, tok, err)
			}
			program = append(program, byte(v))
			pending--
			continue
		}

		op, ok := opcodes[strings.ToLower(tok)]
		if !ok {
			return []byte{}, fmt.Errorf("unknown instruction %q", tok)
		}
		program = append(program, op)
		current = tok
		pending = instructions[op].operands
	}

	if pending > 0 {
		return nil, fmt.Errorf("%s: missing operand", current)
	}

	return program, nil
}
