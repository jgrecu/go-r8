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

	lines := processData(data)
	program := r8.Assemble(lines)

	err = os.WriteFile("program.bin", program, 0o644)
	if err != nil {
		fmt.Println(err)
	}
}

func processData(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	content = strings.Trim(content, "\n")
	lines := strings.Split(content, "\n")
	return lines
}
