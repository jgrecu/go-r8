package r8_test

import (
	"fmt"

	"github.com/jgrecu/go-r8"
)

func ExampleAssemble() {
	program, err := r8.Assemble("ld 0x03\ndec\nbne 0xFD ; loop until A is zero\nhalt")
	if err != nil {
		panic(err)
	}
	fmt.Printf("% X\n", program)
	// Output: 10 03 40 F2 FD 00
}

func ExampleAssemble_labels() {
	program, err := r8.Assemble(`
		ld 0x03
	loop:	dec
		bne loop ; assembles to the offset 0xFD, or -3
		halt`)
	if err != nil {
		panic(err)
	}
	fmt.Printf("% X\n", program)
	// Output: 10 03 40 F2 FD 00
}

func ExampleDisassemble() {
	fmt.Println(r8.Disassemble([]byte{0x10, 0x03, 0x40, 0xF2, 0xFD, 0x00}))
	// Output:
	// ld 0x03
	// dec
	// bne 0xFD ; bne -3
	// halt
}

func ExampleCPU_RunN() {
	program, err := r8.Assemble("ld 0xFA\ninc\nbne 0xFD\nhalt")
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
	fmt.Println(cpu.R[r8.A], cpu.Halted)
	// Output: 0 true
}
