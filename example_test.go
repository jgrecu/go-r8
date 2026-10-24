package r8_test

import (
	"fmt"

	"github.com/jgrecu/go-r8"
)

func ExampleAssemble() {
	program, err := r8.Assemble("ld 3\ndec\nbne -3 ; loop until A is zero\nhalt")
	if err != nil {
		panic(err)
	}
	fmt.Printf("% X\n", program)
	// Output: 10 03 40 F2 FD 00
}

func ExampleDisassemble() {
	fmt.Println(r8.Disassemble([]byte{0x10, 0x03, 0x40, 0xF2, 0xFD, 0x00}))
	// Output:
	// ld 3
	// dec
	// bne -3
	// halt
}

func ExampleCPU_RunN() {
	program, err := r8.Assemble("ld 250\ninc\nbne -3\nhalt")
	if err != nil {
		panic(err)
	}
	cpu := r8.NewCPU()
	if err := cpu.LoadProgram(program, 0); err != nil {
		panic(err)
	}
	if err := cpu.RunN(1000); err != nil {
		panic(err)
	}
	fmt.Println(cpu.A, cpu.Halted)
	// Output: 0 true
}
