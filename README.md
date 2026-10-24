# Emergency in space

![Gophers floating in space](img/space-gophers.webp)

Eleven hours and sixteen minutes ago, our scientists lost contact with the Trailblazer 1 probe en route to Proxima Centauri. Since then we've been trying without success to re-establish contact with the spacecraft's onboard computer. If we can't rectify the problem, we may be faced with a total loss of the vehicle and mission.

That's why we need your help.

## Classified briefing

Welcome to the Situation Room. Thank you for coming in at such short notice. Do you have coffee? Okay, then let's get right to it, because time is a factor.

For the last 15 years Trailblazer has been sailing serenely through interstellar space, sending back valuable science data and engineering telemetry, as the mission team tested, checked, and rechecked all the onboard instruments in preparation for the probe's flyby through the Prox Cen system and its many interesting exoplanets.

Then everything went dead.

The binary data modulated onto the optical carrier signal suddenly changed from valid telemetry data to a meaningless string of 1s. Since we're still receiving the signal, we know the spacecraft is intact, powered, and with its communication laser aimed correctly towards Earth. Doppler radar data indicates that its trajectory is unchanged, making it unlikely that the craft was hit by a meteorite or space debris. It's just no longer able to talk to us.

The problem appears to be localised to a subsystem known as the _RTFM_ (Remote Telemetry and Flight Management) computer, responsible for the communications link between the spacecraft and Earth. Engineers are speculating that its memory may have been corrupted by a cosmic ray impact, causing the software to malfunction.

That's where you come in. We urgently need your help to investigate the RTFM software issue. Specifically, the first thing we need is a Go program that can execute code written for the spacecraft's onboard computer. A program like this is called an **emulator**: it acts like a model of the computer's central processing unit (**CPU**), and will enable us to write and test code before uploading it to the RTFM for real.

You might also have heard the term **virtual machine**, meaning not “nearly a machine”, but something more like “not physically a machine, but behaving in the same way”. Emulator, virtual machine, same thing.

It may sound complicated, but emulators are actually fairly easy to write, once you understand how the emulated machine (the **guest**) works. The RTFM computer uses a relatively small and simple CPU, compared to the one in your computer (the **host** machine).

This CPU is called the **R8**, and to emulate it, you'll be writing the necessary Go code to implement the R8's **instruction set**: the commands it understands.

The first thing the emulator needs is a properly-initialised CPU, ready to work. So there's a `r8.NewCPU()` function that returns a CPU in its default initial state, which is specified by a test.

The test is in the file [`r8_test.go`](r8_test.go), and the `NewCPU` function is already implemented in the [`r8.go`](r8.go) file, so your first challenge is a straightforward one:

**GOAL:** Run the test and make sure it passes.

That's a great start! For the next challenge, read the tutorial:

* [Welcome to the machine: emulating a CPU](https://bitfieldconsulting.com/posts/welcome-to-machine)

## Usage

[![CI](https://github.com/jgrecu/go-r8/actions/workflows/ci.yml/badge.svg)](https://github.com/jgrecu/go-r8/actions/workflows/ci.yml)

The repository has the `r8` emulator package plus three commands for working with R8 programs. The CPU and its assembly language are specified in the R8 Programmer's Manual; this emulator implements a subset of it so far.

### Instruction set

| Mnemonic | Opcode | Operand              | Effect                              |
| -------- | ------ | -------------------- | ----------------------------------- |
| `halt`   | `0x00` |                      | stop execution                      |
| `nop`    | `0x01` |                      | do nothing                          |
| `ld`     | `0x10` | byte                 | load the operand into A             |
| `inc`    | `0x30` |                      | increment A                         |
| `dec`    | `0x40` |                      | decrement A                         |
| `bra`    | `0xF0` | signed byte or label | branch always                       |
| `beq`    | `0xF1` | signed byte or label | branch if A is zero                 |
| `bne`    | `0xF2` | signed byte or label | branch if A is not zero             |
| `jmp`    | `0xF7` | address or label     | jump to the 16-bit address          |

Branch offsets count from the start of the next instruction.

### Assembly syntax

```asm
        ld 0x0A   ; load 10 into A
loop:   dec       ; decrement A
        bne loop  ; repeat until A is zero
        halt      ; stop
```

* Text after `;` is a comment.
* Mnemonics are case-insensitive. Whitespace only separates tokens, and commas separate operands with or without spaces.
* As in the R8 manual, numbers must be `0x`-prefixed hex (`0x0A`). A raw branch offset is a two's complement byte, so `0xFD` means -3.
* `name:` defines a label at the address of the next instruction. Addresses start at 0. Label names are case-sensitive.
* A branch that names a label assembles to that label's offset. A `jmp` that names one assembles to its address.
* Each error message starts with the line number, for example `line 3: bne: bad operand "lop": undefined label`.

### Commands

```sh
# Assemble source into machine code (writes program.bin by default; "-o -" writes to stdout).
go run ./cmd/assembler [-o program.bin] program.asm

# Disassemble machine code back into source (writes program.src by default).
go run ./cmd/disassembler [-o program.src] program.bin

# Load a program at address 0 and step one instruction each time you press Enter.
go run ./cmd/monitor program.bin
```

For example, the countdown loop above is in [`program.asm`](program.asm): assemble, disassemble and step through it with the commands as shown. In the disassembler's output, the label shows up as the raw offset with its signed value in a comment: `bne 0xFD ; bne -3`. The generated `*.bin` and `*.src` files are gitignored.

### Library

```go
program, err := r8.Assemble("ld 0x03\nloop: dec\nbne loop\nhalt")
if err != nil {
	log.Fatal(err)
}
cpu := r8.NewCPU()
if err := cpu.LoadProgram(program, 0); err != nil {
	log.Fatal(err)
}
if err := cpu.RunN(1000); err != nil { // ErrStepLimit if it never halts
	log.Fatal(err)
}
fmt.Println(cpu.R[r8.A]) // 0
```

`Step` runs one instruction, `Run` runs until `halt`, and `Disassemble` converts machine code back into source. See the package docs (`go doc -all .`) for details.

### Development

```sh
gofmt -l .
go vet ./...
go test -race ./...
```

CI runs these same checks on every push to `main` and on every pull request.
