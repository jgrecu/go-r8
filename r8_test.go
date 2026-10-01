package r8_test

import (
	"bytes"
	"os"
	"path/filepath"
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
	cpu.LoadProgram([]byte{r8.NOP, r8.NOP})
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
	cpu.LoadProgram([]byte{r8.INC, r8.HALT})
	cpu.Run()
	if cpu.A != 1 {
		t.Errorf("want A == 1, got %d", cpu.A)
	}
}

func TestHaltStopsCPU(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram([]byte{r8.NOP, r8.HALT})
	cpu.Run()
	if cpu.PC != 2 {
		t.Errorf("want PC == 2, got %d", cpu.PC)
	}
}

func TestDecDecrementsA(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram([]byte{r8.INC, r8.DEC, r8.HALT})
	cpu.Run()
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
	}
}

func TestIncWrapsAFrom255To0(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram([]byte{r8.INC, r8.HALT})
	cpu.A = 255
	cpu.Run()
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
	}
}

func TestDecWrapsAFrom0To255(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram([]byte{r8.DEC, r8.HALT})
	cpu.A = 0
	cpu.Run()
	if cpu.A != 255 {
		t.Errorf("want A == 255, got %d", cpu.A)
	}
}

func TestStepWrapsPCFrom65535To0(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.LoadProgram([]byte{r8.NOP, r8.HALT})
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
