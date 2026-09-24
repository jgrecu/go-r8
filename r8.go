// Package r8 emulates a simple CPU called the R8.
package r8

type CPU struct {
	PC  int
	Mem [65536]int
	A   int
}

const (
	HALT = 0x00
	NOP  = 0x01
	INC  = 0x30
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
