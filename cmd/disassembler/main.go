// Command disassembler translates R8 machine code into assembly source.
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
		fmt.Fprintln(os.Stderr, "disassembler:", err)
		os.Exit(1)
	}
}

// run disassembles the machine code file named in args and writes the
// source to the -o file, or to stdout if it is "-".
func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("disassembler", flag.ContinueOnError)
	out := flags.String("o", "program.src", `output file, or "-" for stdout`)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: disassembler [-o output] program.bin")
	}
	code, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return err
	}
	src := r8.Disassemble(code) + "\n"
	if *out == "-" {
		_, err := io.WriteString(stdout, src)
		return err
	}
	return os.WriteFile(*out, []byte(src), 0o644)
}
