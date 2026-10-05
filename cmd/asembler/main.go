package main

import (
	"fmt"
	"os"
	"strings"

	r8 "github.com/jgrecu/go-r8"
)

func main() {
	data, err := os.ReadFile("program.r8")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// should we fail on an empty source file?
	if len(data) == 0 {
		fmt.Fprintln(os.Stdout, "source file is empty")
		os.Exit(0)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	content = strings.Trim(content, "\n")
	lines := strings.Split(content, "\n")

	program := make([]byte, len(lines))
	for i, v := range lines {
		program[i] = r8.Asemble(strings.ToLower(v))
	}

	//program := []byte{r8.INC, r8.INC, r8.DEC, r8.DEC, r8.HALT}
	err = os.WriteFile("program.bin", program, 0o644)
	if err != nil {
		fmt.Println(err)
	}
}
