package main

import (
	"fmt"
	"os"

	r8 "github.com/jgrecu/go-r8"
)

func main() {
	program := []byte{r8.INC, r8.INC, r8.DEC, r8.DEC, r8.HALT}
	err := os.WriteFile("program.bin", program, 0o644)
	if err != nil {
		fmt.Println(err)
	}
}
