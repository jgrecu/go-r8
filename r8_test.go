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
	cpu := r8.NewCPU()
	cpu.LoadProgram(r8.Assemble([]string{"NOP", "NOP"}))
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
	cpu := r8.NewCPU()
	cpu.LoadProgram(r8.Assemble([]string{"INC", "HALT"}))
	cpu.Run()
	if cpu.A != 1 {
		t.Errorf("want A == 1, got %d", cpu.A)
	}
}

func TestHaltStopsCPU(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram(r8.Assemble([]string{"NOP", "HALT"}))
	cpu.Run()
	if cpu.PC != 2 {
		t.Errorf("want PC == 2, got %d", cpu.PC)
	}
}

func TestDecDecrementsA(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram(r8.Assemble([]string{"INC", "DEC", "HALT"}))
	cpu.Run()
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
	}
}

func TestIncWrapsAFrom255To0(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram(r8.Assemble([]string{"INC", "HALT"}))
	cpu.A = 255
	cpu.Run()
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
	}
}

func TestDecWrapsAFrom0To255(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram(r8.Assemble([]string{"DEC", "HALT"}))
	cpu.A = 0
	cpu.Run()
	if cpu.A != 255 {
		t.Errorf("want A == 255, got %d", cpu.A)
	}
}

func TestStepWrapsPCFrom65535To0(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram(r8.Assemble([]string{"NOP", "HALT"}))
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

func TestAssembleBasic(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []byte
	}{
		{"empty", []string{}, []byte{}},
		{"nop", []string{"NOP"}, []byte{0x01}},
		{"inc", []string{"INC"}, []byte{0x30}},
		{"dec", []string{"DEC"}, []byte{0x40}},
		{"halt", []string{"HALT"}, []byte{0x00}},
		{"mixedcase", []string{"nop", "Inc", "dEc", "HaLt"}, []byte{0x01, 0x30, 0x40, 0x00}},
		{"sequence", []string{"INC", "INC", "DEC", "HALT"}, []byte{0x30, 0x30, 0x40, 0x00}},
		{"unknown", []string{"FOOBAR"}, []byte{0x01}}, // current behavior: defaults to NOP
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r8.Assemble(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Assemble(%v)=%v want %v", c.in, got, c.want)
			}
		})
	}
}

func TestDisassembleBasic(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []string
	}{
		{"empty", []byte{}, []string{}},
		{"nop", []byte{0x01}, []string{"nop"}},
		{"inc", []byte{0x30}, []string{"inc"}},
		{"dec", []byte{0x40}, []string{"dec"}},
		{"halt", []byte{0x00}, []string{"halt"}},
		{"sequence", []byte{0x30, 0x30, 0x40, 0x00}, []string{"inc", "inc", "dec", "halt"}},
		{"unknown", []byte{0x02}, []string{"unimplemented"}},
		{"unknown2", []byte{0xFF}, []string{"unimplemented"}},
		{"mixed", []byte{0x01, 0x02, 0x00}, []string{"nop", "unimplemented", "halt"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r8.Disassemble(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Disassemble(%v)=%v want %v", c.in, got, c.want)
			}
		})
	}
}

func TestAssembleDisassembleRoundtrip(t *testing.T) {
	cases := [][]string{
		{},
		{"NOP"},
		{"INC", "DEC", "HALT"},
		{"nop", "inc", "dec", "halt"},
		{"INC", "INC", "INC"},
	}
	for _, c := range cases {
		asm := r8.Assemble(c)
		dasm := r8.Disassemble(asm)
		// canonicalize input to lowercase for comparison
		want := make([]string, len(c))
		for i := range c {
			want[i] = strings.ToLower(c[i])
		}
		if !reflect.DeepEqual(dasm, want) {
			t.Errorf("roundtrip %v -> dasm=%v want %v", c, dasm, want)
		}
	}
}
