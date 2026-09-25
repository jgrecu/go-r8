// Package r8 emulates a simple CPU called the R8.
package r8

// https://github.com/bitfield/r8/blob/main/crates/rx82/README.md#the-rx82-architecture
// R8 CPU clocked at 4Mhz, 64KiB of static RAM, an 8-bit data bus, and a 16-bit address bus.
type CPU struct {
	PC  uint16
	Mem [65536]int
	A   byte
}

const (
	HALT = 0x00
	NOP  = 0x01
	INC  = 0x30
	DEC  = 0x40
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
