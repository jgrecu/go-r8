package main

import (
	"bufio"
	"fmt"
	"os"

	r8 "github.com/jgrecu/go-r8"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "you need the file name as first parameter")
		os.Exit(2)
	}
	cpu := r8.NewCPU()
	if err := cpu.LoadFile(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scan := bufio.NewScanner(os.Stdin)

	fmt.Println("Next Oper      PC   > A")
	fmt.Print(cpu)
	for scan.Scan() {
		if !cpu.Step() {
			break
		}
		fmt.Print(cpu)
	}
	if err := scan.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println()
}
