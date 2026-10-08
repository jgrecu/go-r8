package main

import (
	"bufio"
	"fmt"
	"os"

	r8 "github.com/jgrecu/go-r8"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("you need the file name as first paramater")
		return
	}
	cpu := r8.NewCPU()
	cpu.LoadFile(os.Args[1])
	scan := bufio.NewScanner(os.Stdin)

	fmt.Println("Next Oper      PC   > A")
	fmt.Print(cpu)
	for scan.Scan() {
		cpu.Step()
		fmt.Print(cpu)
		if scan.Err() != nil {
			break
		}
		if cpu.Mem[cpu.PC] == 0 {
			break
		}
	}
	fmt.Println()
}
