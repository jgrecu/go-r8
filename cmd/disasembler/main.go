package main

import (
	"fmt"
	"os"
	"strings"

	r8 "github.com/jgrecu/go-r8"
)

func main() {
	data, err := os.ReadFile("program.bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// should we fail on an empty source file?
	if len(data) == 0 {
		fmt.Fprintln(os.Stdout, "binary file is empty")
		os.Exit(0)
	}

	lines := make([]string, len(data))
	for i, v := range data {
		lines[i] = r8.Disasemble(v)
	}

	srcData := []byte(strings.Join(lines, "\n") + "\n")

	err = os.WriteFile("program.src", srcData, 0o644)
	if err != nil {
		fmt.Println(err)
	}
}
