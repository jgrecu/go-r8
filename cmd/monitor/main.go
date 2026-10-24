// Command monitor loads an R8 program and single-steps it, executing one
// instruction each time a line is read from standard input.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	r8 "github.com/jgrecu/go-r8"
)

const header = "Next Oper            PC      > A"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "monitor:", err)
		os.Exit(1)
	}
}

// run loads the program named in args at address 0, then prints the CPU
// state and steps once per line read from in, until the CPU halts or in is
// exhausted.
func run(args []string, in io.Reader, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: monitor program.bin")
	}
	program, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	cpu := r8.NewCPU()
	if err := cpu.LoadProgram(program, 0); err != nil {
		return fmt.Errorf("load %s: %w", args[0], err)
	}

	fmt.Fprintln(out, header)
	fmt.Fprint(out, status(cpu))
	scan := bufio.NewScanner(in)
	for !cpu.Halted && scan.Scan() {
		if err := cpu.Step(); err != nil {
			return err
		}
		fmt.Fprint(out, status(cpu))
	}
	fmt.Fprintln(out)
	return scan.Err()
}

// status returns the monitor's one-line view of the next instruction, PC,
// and A. It has no trailing newline: at a terminal, the user's Enter key
// supplies it.
func status(cpu *r8.CPU) string {
	// Only the mnemonic is uppercased: the assembler reads "0x" but not "0X".
	mnemonic, operand, _ := strings.Cut(cpu.NextInstruction(), " ")
	next := strings.TrimSpace(strings.ToUpper(mnemonic) + " " + operand)
	label := fmt.Sprintf("next %-12q", next)
	if cpu.Halted {
		label = "halted"
	}
	return fmt.Sprintf("%-17s => %07d > %04d", label, cpu.PC, cpu.R[r8.A])
}
