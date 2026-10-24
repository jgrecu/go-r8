// Package r8 emulates a simple CPU called the R8.
package r8

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// CPU is an R8 CPU clocked at 4MHz, with 64KiB of static RAM, an 8-bit data
// bus, and a 16-bit address bus. The zero value is a CPU in its initial state.
//
// See https://github.com/bitfield/r8/blob/main/crates/rx82/README.md#the-rx82-architecture
type CPU struct {
	// PC is the program counter: the address of the next byte to execute.
	PC uint16
	// A is the accumulator register.
	A byte
	// Memory is the 64KB CPU's addressable memory.
	Memory [MemSize]byte
	// Halted is true once a HALT instruction has executed.
	Halted bool
}

// Opcodes of the R8 instruction set.
const (
	OpHalt byte = 0x00 // stop execution
	OpNop  byte = 0x01 // do nothing
	OpInc  byte = 0x30 // increment A
	OpDec  byte = 0x40 // decrement A
	OpLd   byte = 0x10 // load the following byte into A
	OpJmp  byte = 0xF7 // jump to the following 16-bit little-endian address
	OpBra  byte = 0xF0 // branch by the following signed byte, relative to the next instruction
	OpBeq  byte = 0xF1 // branch by the following signed byte if A is zero
	OpBne  byte = 0xF2 // branch by the following signed byte if A is not zero
)

// MemSize is the size of the CPU's memory in bytes.
const MemSize = 65536

// Errors returned by LoadProgram, RunN, and Assemble. Use errors.Is to
// check for them, since they are usually wrapped with more context.
var (
	ErrProgramTooLarge    = errors.New("program does not fit into memory")
	ErrStepLimit          = errors.New("step limit reached")
	ErrUnknownInstruction = errors.New("unknown instruction")
	ErrBadOperand         = errors.New("bad operand")
	ErrMissingOperand     = errors.New("missing operand")
	ErrUndefinedLabel     = errors.New("undefined label")
	ErrDuplicateLabel     = errors.New("duplicate label")
	ErrBadLabel           = errors.New("bad label name")
)

// IllegalInstructionError reports an undefined opcode and the address it was
// fetched from.
type IllegalInstructionError struct {
	Opcode byte
	PC     uint16
}

func (e *IllegalInstructionError) Error() string {
	return fmt.Sprintf("illegal instruction: 0x%02X at PC=0x%04X", e.Opcode, e.PC)
}

// operandKind says how the bytes following an opcode are encoded, and is the
// single place that knows how to read, write, parse, and print them.
type operandKind uint8

const (
	none   operandKind = iota
	imm8               // unsigned byte
	rel8               // signed byte offset, relative to the next instruction
	addr16             // little-endian 16-bit address
)

// size returns the width of the operand in bytes.
func (k operandKind) size() int {
	switch k {
	case imm8, rel8:
		return 1
	case addr16:
		return 2
	}
	return 0
}

// decode returns the value of the operand encoded in b, which holds exactly
// k.size() bytes.
func (k operandKind) decode(b []byte) uint16 {
	switch k {
	case imm8, rel8:
		return uint16(b[0])
	case addr16:
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

// encode appends the encoding of v to code.
func (k operandKind) encode(code []byte, v uint16) []byte {
	switch k {
	case imm8, rel8:
		return append(code, byte(v))
	case addr16:
		return binary.LittleEndian.AppendUint16(code, v)
	}
	return code
}

// parse converts a literal to an operand value. As in the R8 manual, only
// 0x-prefixed hex is recognised. A rel8 offset is written as its two's
// complement byte, so 0xFD branches back 3.
func (k operandKind) parse(s string) (uint16, error) {
	digits, ok := strings.CutPrefix(s, "0x")
	if !ok {
		return 0, errors.New("literals must be 0x-prefixed hex")
	}
	v, err := strconv.ParseUint(digits, 16, 8*k.size())
	return uint16(v), err
}

// resolve returns the value of operand token tok, which is a number or a
// label. A label gives an absolute address for addr16, or an offset from
// next, the address of the following instruction, for rel8.
func (k operandKind) resolve(tok string, labels map[string]int, next int) (uint16, error) {
	if !isLabel(tok) {
		return k.parse(tok)
	}
	if k != addr16 && k != rel8 {
		return 0, errors.New("labels are only allowed for jumps and branches")
	}
	addr, ok := labels[tok]
	if !ok {
		return 0, ErrUndefinedLabel
	}
	if k == addr16 {
		return uint16(addr), nil
	}
	off := addr - next
	if off < math.MinInt8 || off > math.MaxInt8 {
		return 0, fmt.Errorf("label %q is %d bytes away, beyond %d to %d",
			tok, off, math.MinInt8, math.MaxInt8)
	}
	return uint16(uint8(int8(off))), nil
}

// isLabel reports whether s is a valid label name: a letter or underscore,
// then any number of letters, digits, and underscores.
func isLabel(s string) bool {
	for i, r := range s {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return s != ""
}

// format returns the source text for v as 0x-prefixed hex: two digits for
// bytes and offsets, four for addresses.
func (k operandKind) format(v uint16) string {
	if k == addr16 {
		return fmt.Sprintf("0x%04X", v)
	}
	return fmt.Sprintf("0x%02X", v)
}

// instruction describes one opcode. execute runs after Step has fetched the
// operand and moved PC to the next instruction, so it holds only the
// instruction's semantics.
type instruction struct {
	mnemonic string
	operand  operandKind
	execute  func(cpu *CPU, operand uint16)
}

// source returns the instruction with operand v as one line of source.
func (inst *instruction) source(v uint16) string {
	if inst.operand == none {
		return inst.mnemonic
	}
	return inst.mnemonic + " " + inst.operand.format(v)
}

// instructions is the decode table, indexed by opcode. A nil entry is an
// illegal instruction.
var instructions = [256]*instruction{
	OpHalt: {"halt", none, func(cpu *CPU, _ uint16) { cpu.Halted = true }},
	OpNop:  {"nop", none, func(*CPU, uint16) {}},
	OpInc:  {"inc", none, func(cpu *CPU, _ uint16) { cpu.A++ }},
	OpDec:  {"dec", none, func(cpu *CPU, _ uint16) { cpu.A-- }},
	OpLd:   {"ld", imm8, func(cpu *CPU, v uint16) { cpu.A = byte(v) }},
	OpJmp:  {"jmp", addr16, func(cpu *CPU, addr uint16) { cpu.PC = addr }},
	OpBra:  {"bra", rel8, func(cpu *CPU, off uint16) { cpu.branchIf(true, off) }},
	OpBeq:  {"beq", rel8, func(cpu *CPU, off uint16) { cpu.branchIf(cpu.A == 0, off) }},
	OpBne:  {"bne", rel8, func(cpu *CPU, off uint16) { cpu.branchIf(cpu.A != 0, off) }},
}

var opcodes = func() map[string]byte {
	m := make(map[string]byte)
	for op, inst := range instructions {
		if inst != nil {
			m[inst.mnemonic] = byte(op)
		}
	}
	return m
}()

// branchIf moves PC by the signed offset (-128 to 127) if cond is true. The
// target wraps around memory in either direction.
func (cpu *CPU) branchIf(cond bool, offset uint16) {
	if cond {
		cpu.PC += uint16(int8(offset))
	}
}

// NewCPU returns a CPU in its initial state.
func NewCPU() *CPU {
	return &CPU{}
}

// Step executes the instruction at PC. It does nothing once the CPU is
// halted. On an unknown opcode it moves PC past it and returns an
// *IllegalInstructionError.
func (cpu *CPU) Step() error {
	if cpu.Halted {
		return nil
	}
	opcode := cpu.Memory[cpu.PC]
	inst := instructions[opcode]
	if inst == nil {
		err := &IllegalInstructionError{Opcode: opcode, PC: cpu.PC}
		cpu.PC++
		return err
	}
	operand := cpu.operandAt(cpu.PC+1, inst.operand)
	cpu.PC += 1 + uint16(inst.operand.size())
	inst.execute(cpu, operand)
	return nil
}

// operandAt returns the value of an operand of kind k stored at addr.
// Multi-byte operands wrap around the end of memory like real 16-bit hardware.
func (cpu *CPU) operandAt(addr uint16, k operandKind) uint16 {
	var b [2]byte
	for i := range k.size() {
		b[i] = cpu.Memory[addr+uint16(i)]
	}
	return k.decode(b[:k.size()])
}

// Run executes instructions until the CPU halts or Step returns an error.
func (cpu *CPU) Run() error {
	for !cpu.Halted {
		if err := cpu.Step(); err != nil {
			return err
		}
	}
	return nil
}

// RunN executes at most n instructions, stopping early if the CPU halts or
// Step returns an error. It returns ErrStepLimit if the CPU is still running
// after n steps, which guards against programs that never halt.
func (cpu *CPU) RunN(n int) error {
	for range n {
		if cpu.Halted {
			return nil
		}
		if err := cpu.Step(); err != nil {
			return err
		}
	}
	if !cpu.Halted {
		return fmt.Errorf("%w: %d", ErrStepLimit, n)
	}
	return nil
}

// LoadProgram copies program into memory starting at addr and sets PC to
// addr. It returns ErrProgramTooLarge, leaving the CPU unchanged, if program
// does not fit between addr and the end of memory.
func (cpu *CPU) LoadProgram(program []byte, addr uint16) error {
	availableSpace := MemSize - int(addr)
	if len(program) > availableSpace {
		return fmt.Errorf("%w: %d bytes at 0x%04X exceeds memory (available: %d bytes)",
			ErrProgramTooLarge, len(program), addr, availableSpace)
	}
	copy(cpu.Memory[addr:], program)
	cpu.PC = addr
	return nil
}

// NextInstruction returns the source form of the instruction at PC, such as
// "ld 5" or "halt", or "???" if the opcode is unknown. Operands that run past
// the end of memory wrap around, as they do in Step.
func (cpu *CPU) NextInstruction() string {
	inst := instructions[cpu.Memory[cpu.PC]]
	if inst == nil {
		return "???"
	}
	return inst.source(cpu.operandAt(cpu.PC+1, inst.operand))
}

// Disassemble converts machine code into one line of source per instruction.
// Operands are shown as 0x-prefixed hex, and a branch is followed by a
// comment giving its signed offset, as in "bne 0xFD ; bne -3". Unknown
// opcodes are shown as "???", and a final instruction missing its operands
// as its mnemonic followed by "???".
func Disassemble(code []byte) string {
	var lines []string
	for i := 0; i < len(code); {
		inst := instructions[code[i]]
		i++
		if inst == nil {
			lines = append(lines, "???")
			continue
		}
		end := i + inst.operand.size()
		if end > len(code) {
			lines = append(lines, inst.mnemonic+" ???")
			break
		}
		v := inst.operand.decode(code[i:end])
		line := inst.source(v)
		if inst.operand == rel8 {
			line += " ; " + inst.mnemonic + " " + strconv.Itoa(int(int8(v)))
		}
		lines = append(lines, line)
		i = end
	}
	return strings.Join(lines, "\n")
}

// Assemble converts source text into machine code. Each line holds zero or
// more instructions, each followed by its operand on the same line; text
// after ";" is a comment. Whitespace only separates tokens, and a comma
// separates operands with or without spaces around it. Mnemonics are
// case-insensitive. As in the R8 manual, numeric operands must be
// 0x-prefixed hex; a branch offset is a two's complement byte, so 0xFD
// branches back 3. Multi-byte operands are stored little-endian.
//
// A token ending in ":", such as "loop:", defines a label at the address of
// the next instruction, counting from 0. Label names are case-sensitive,
// start with a letter or underscore, and continue with letters, digits, and
// underscores. A jump operand may name a label to use its address, and a
// branch operand to use its offset; labels are not allowed elsewhere.
//
// Errors start with the line number and wrap one of the package's Err values.
func Assemble(src string) ([]byte, error) {
	stmts, labels, err := parse(src)
	if err != nil {
		return nil, err
	}
	return encode(stmts, labels)
}

// parse is the assembler's first pass: it splits src into statements and
// records the address of each label.
func parse(src string) ([]stmt, map[string]int, error) {
	p := parser{labels: make(map[string]int)}
	for n, line := range strings.Split(src, "\n") {
		code, _, _ := strings.Cut(line, ";")
		if err := p.parseLine(n+1, tokenize(code)); err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", n+1, err)
		}
	}
	return p.stmts, p.labels, nil
}

// tokenize splits one line of code into tokens. Whitespace only separates
// tokens, and a comma is a token of its own with or without spaces around it.
func tokenize(code string) []string {
	return strings.Fields(strings.ReplaceAll(code, ",", " , "))
}

// encode is the assembler's second pass: it emits the machine code for
// stmts, resolving operands that name labels.
func encode(stmts []stmt, labels map[string]int) ([]byte, error) {
	var program []byte
	for _, s := range stmts {
		inst := instructions[s.op]
		program = append(program, s.op)
		if inst.operand == none {
			continue
		}
		next := len(program) + inst.operand.size()
		v, err := inst.operand.resolve(s.operand, labels, next)
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w %q: %w",
				s.line, inst.mnemonic, ErrBadOperand, s.operand, err)
		}
		program = inst.operand.encode(program, v)
	}
	return program, nil
}

// stmt is one instruction from the source, with its operand not yet resolved.
type stmt struct {
	line    int    // source line number, for error messages
	op      byte   // opcode
	operand string // operand token, empty if the instruction takes none
}

// parser holds the state of the first pass. It can assign label addresses
// before reading any operand because every instruction's size follows from
// its opcode alone.
type parser struct {
	stmts  []stmt
	labels map[string]int
	pc     int // address of the next instruction
}

// parseLine adds the labels and instructions in one line's tokens.
func (p *parser) parseLine(line int, tokens []string) error {
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if name, ok := strings.CutSuffix(tok, ":"); ok {
			if err := p.define(name); err != nil {
				return err
			}
			continue
		}
		op, ok := opcodes[strings.ToLower(tok)]
		if !ok {
			return fmt.Errorf("%w: %q", ErrUnknownInstruction, tok)
		}
		s := stmt{line: line, op: op}

		kind := instructions[op].operand
		if kind != none {
			i++
			if i == len(tokens) || tokens[i] == "," {
				return fmt.Errorf("%s: %w", tok, ErrMissingOperand)
			}
			s.operand = tokens[i]
		}
		if i+1 < len(tokens) && tokens[i+1] == "," {
			return fmt.Errorf("%s: %w: too many operands", tok, ErrBadOperand)
		}
		p.pc += 1 + kind.size()
		if p.pc > MemSize {
			return fmt.Errorf("%w: %d bytes", ErrProgramTooLarge, p.pc)
		}
		p.stmts = append(p.stmts, s)
	}
	return nil
}

// define records label name at the current address.
func (p *parser) define(name string) error {
	if !isLabel(name) {
		return fmt.Errorf("%w: %q", ErrBadLabel, name)
	}
	if _, dup := p.labels[name]; dup {
		return fmt.Errorf("%w: %q", ErrDuplicateLabel, name)
	}
	p.labels[name] = p.pc
	return nil
}
