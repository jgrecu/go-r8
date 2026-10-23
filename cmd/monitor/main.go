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
	if err := loadFile(cpu, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scan := bufio.NewScanner(os.Stdin)

	fmt.Println("Next Oper            PC   > A")
	fmt.Print(cpu)
	for scan.Scan() {
		if err := cpu.Step(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if cpu.Halted {
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

// loadFile reads the program at path and loads it into cpu's memory.
func loadFile(cpu *r8.CPU, path string) error {
	program, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := cpu.LoadProgram(program, 0); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	return nil
}
