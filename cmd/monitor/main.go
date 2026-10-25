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

// status returns the monitor's one-line view of the CPU state, followed by
// the next instruction. It has no trailing newline: at a terminal, the
// user's Enter key supplies it.
func status(cpu *r8.CPU) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CPU state: PC: %04X  ", cpu.PC)
	for r, v := range cpu.R {
		fmt.Fprintf(&b, "%s: %02X  ", strings.ToUpper(r8.Register(r).String()), v)
	}
	fmt.Fprintf(&b, "PS: %v  Next op: %s", cpu.PS, next(cpu))
	return b.String()
}

// next returns the instruction at PC, or "halted". Mnemonics and registers
// are uppercased, but numbers are not: the assembler reads "0x" but not "0X".
func next(cpu *r8.CPU) string {
	if cpu.Halted {
		return "halted"
	}
	words := strings.Fields(cpu.NextInstruction())
	for i, w := range words {
		if !strings.HasPrefix(w, "0x") {
			words[i] = strings.ToUpper(w)
		}
	}
	return strings.Join(words, " ")
}
