// Command assembler translates R8 assembly source into machine code.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	r8 "github.com/jgrecu/go-r8"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "assembler:", err)
		os.Exit(1)
	}
}

// run assembles the source file named in args and writes the machine code
// to the -o file, or to stdout if it is "-".
func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("assembler", flag.ContinueOnError)
	out := flags.String("o", "program.bin", `output file, or "-" for stdout`)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: assembler [-o output] source")
	}
	src, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return err
	}
	program, err := r8.Assemble(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", flags.Arg(0), err)
	}
	if *out == "-" {
		_, err := stdout.Write(program)
		return err
	}
	return os.WriteFile(*out, program, 0o644)
}
