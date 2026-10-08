package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jgrecu/go-r8"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("you need the file name as first paramater")
		return
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lines := strings.Fields(string(data))
	program, err := r8.Assemble(lines)
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("source file error: %w", err))
		os.Exit(1)
	}

	err = os.WriteFile("program.bin", program, 0o644)
	if err != nil {
		fmt.Println(err)
	}
}
