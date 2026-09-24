// Package r8 emulates a simple CPU called the R8.
package r8

type CPU struct {
	PC  int
	Mem [65536]int
	A   int
}

const (
	NOP = 0x01
	INC = 0x30
)

func NewCPU() *CPU {
	return &CPU{}
}

func (cpu *CPU) Step() {
	op := cpu.Mem[cpu.PC]
	if op == INC {
		cpu.A++
	}
	cpu.PC++
}
