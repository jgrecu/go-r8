package main

import (
	"fmt"
	"os"
	"strings"

	r8 "github.com/jgrecu/go-r8"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "you need the file name as first parameter")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lines := r8.Disassemble(data)
	srcData := []byte(strings.Join(lines, "\n") + "\n")

	err = os.WriteFile("program.src", srcData, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
