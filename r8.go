// Package r8 emulates a simple CPU called the R8.
package r8

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// https://github.com/bitfield/r8/blob/main/crates/rx82/README.md#the-rx82-architecture
// R8 CPU clocked at 4Mhz, 64KiB of static RAM, an 8-bit data bus, and a 16-bit address bus.
type CPU struct {
	PC  uint16
	A   byte
	Mem [MEMSIZE]byte
}

const (
	HALT byte = 0x00
	NOP  byte = 0x01
	INC  byte = 0x30
	DEC  byte = 0x40

	MEMSIZE = 65536
)

func NewCPU() *CPU {
	return &CPU{}
}

func (cpu *CPU) Step() bool {
	opcode := cpu.Mem[cpu.PC]
	cpu.PC++
	switch opcode {
	case INC:
		cpu.A++
	case DEC:
		cpu.A--
	case NOP:
	// No operation to do
	case HALT:
		return false
	}
	return true
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
	return fmt.Sprintf("next %q => %04d > %07d", strings.ToUpper(Disasemble(cpu.Mem[cpu.PC])), cpu.PC, cpu.A)
}

func Disasemble(b byte) string {
	switch b {
	case NOP:
		return "nop"
	case INC:
		return "inc"
	case DEC:
		return "dec"
	case HALT:
		return "halt"
	default:
		return "unimplemented"
	}
}

func Asemble(s string) byte {
	s = strings.ToLower(s)
	switch s {
	case "nop":
		return NOP
	case "inc":
		return INC
	case "dec":
		return DEC
	case "halt":
		return HALT
	default:
		return NOP
	}
}
