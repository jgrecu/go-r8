package r8_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jgrecu/go-r8"
)

func TestNewInitialisesCPU(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	if cpu.PC != 0 {
		t.Errorf("after New, want pc == 0, got %d", cpu.PC)
	}
	got := cpu.Mem[0]
	if got != 0 {
		t.Errorf("after New, want Memory[0] == 0, got %d", got)
	}

	if cpu.A != 0 {
		t.Errorf("after New, want a == 0, got %d", got)
	}
}

// Uncomment this test once the previous test passes!
func TestStepIncrementsPC(t *testing.T) {
	t.Parallel()
	input := []string{"NOP", "NOP"}
	program, err := r8.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	cpu.LoadProgram(program)
	cpu.Step()
	if cpu.PC != 1 {
		t.Errorf("want pc == 1, got %d", cpu.PC)
	}
	cpu.Step()
	if cpu.PC != 2 {
		t.Errorf("want pc == 2, got %d", cpu.PC)
	}
}

func TestIncIncrementsA(t *testing.T) {
	t.Parallel()
	input := []string{"INC", "HALT"}
	program, err := r8.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	cpu.LoadProgram(program)
	cpu.Run()
	if cpu.A != 1 {
		t.Errorf("want A == 1, got %d", cpu.A)
	}
}

func TestHaltStopsCPU(t *testing.T) {
	t.Parallel()
	input := []string{"NOP", "HALT"}
	program, err := r8.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	cpu.LoadProgram(program)
	cpu.Run()
	if cpu.PC != 2 {
		t.Errorf("want PC == 2, got %d", cpu.PC)
	}
}

func TestDecDecrementsA(t *testing.T) {
	t.Parallel()
	input := []string{"INC", "DEC", "HALT"}
	program, err := r8.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	cpu.LoadProgram(program)
	cpu.Run()
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
	}
}

func TestIncWrapsAFrom255To0(t *testing.T) {
	t.Parallel()
	input := []string{"INC", "HALT"}
	program, err := r8.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	cpu.LoadProgram(program)
	cpu.A = 255
	cpu.Run()
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
	}
}

func TestDecWrapsAFrom0To255(t *testing.T) {
	t.Parallel()
	input := []string{"DEC", "HALT"}
	program, err := r8.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	cpu.LoadProgram(program)
	cpu.A = 0
	cpu.Run()
	if cpu.A != 255 {
		t.Errorf("want A == 255, got %d", cpu.A)
	}
}

func TestStepWrapsPCFrom65535To0(t *testing.T) {
	t.Parallel()
	input := []string{"NOP", "HALT"}
	program, err := r8.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	cpu := r8.NewCPU()
	cpu.LoadProgram(program)
	cpu.PC = 65535
	cpu.Run()
	if cpu.PC != 0 {
		t.Errorf("want PC == 0, got %d", cpu.PC)
	}
}

func TestLoadFileLoadsDataFomFileIntoMemory(t *testing.T) {
	t.Parallel()
	want := []byte{0x30, 0x30, 0x00}
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("failed setting up test file: %v", err)
	}
	cpu := r8.NewCPU()
	cpu.LoadFile(path)
	got := cpu.Mem[0:len(want)]
	if !bytes.Equal(got, want) {
		t.Errorf("want %+v, got %+v", want, got)
	}
}

func TestAssembleDisassemble(t *testing.T) {
	cases := []struct {
		name        string
		in          []string
		want        []byte
		expectError bool
	}{
		{"empty", []string{}, []byte{}, false},
		{"nop", []string{"NOP"}, []byte{0x01}, false},
		{"inc", []string{"INC"}, []byte{0x30}, false},
		{"dec", []string{"DEC"}, []byte{0x40}, false},
		{"halt", []string{"HALT"}, []byte{0x00}, false},
		{"mixedcase", []string{"nop", "Inc", "dEc", "HaLt"}, []byte{0x01, 0x30, 0x40, 0x00}, false},
		{"sequence", []string{"INC", "INC", "DEC", "HALT"}, []byte{0x30, 0x30, 0x40, 0x00}, false},
		{"unknown", []string{"FOOBAR"}, []byte{}, true},
		{"unknown in sequence", []string{"INC", "INC", "FOO", "HALT"}, []byte{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			asm, err := r8.Assemble(c.in)
			if err != nil && !c.expectError {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(asm, c.want) {
				t.Errorf("Assemble(%v)=%v want %v", c.in, asm, c.want)
			}
			dasm := r8.Disassemble(asm)
			var want []string
			if c.expectError {
				want = []string{}
			} else {
				want = make([]string, len(c.in))
				for i := range c.in {
					want[i] = strings.ToLower(c.in[i])
				}
			}

			if !reflect.DeepEqual(dasm, want) {
				t.Errorf("roundtrip %v -> dasm=%v want %v", c.in, dasm, want)
			}
		})
	}
}
