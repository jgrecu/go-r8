        ld 0x0A   ; load 10 into A
loop:
        dec       ; decrement A
        bne loop  ; repeat until A is zero
        halt      ; stop
