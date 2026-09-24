package r8_test

import (
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
	cpu.Mem[0] = r8.NOP
	cpu.Mem[1] = r8.NOP
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
	cpu.Mem[0] = r8.INC
	cpu.Step()
	if cpu.A != 1 {
		t.Errorf("want A == 1, got %d", cpu.A)
	}
}

func TestHaltStopsCPU(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Mem[0] = r8.NOP
	cpu.Mem[2] = r8.HALT
	cpu.Run()
	if cpu.PC != 2 {
		t.Errorf("want PC == 2, got %d", cpu.PC)
	}
}

func TestDecDecrementsA(t *testing.T) {
	t.Parallel()
	cpu := r8.NewCPU()
	cpu.Mem[0] = r8.INC
	cpu.Mem[1] = r8.DEC
	cpu.Step()
	cpu.Step()
	if cpu.A != 0 {
		t.Errorf("want A == 0, got %d", cpu.A)
	}
}
