TEXT ·ctzll(SB), $0-12
    MOVQ x+0(FP), AX
    TZCNTQ AX, AX 
    MOVL AX, ret+8(FP)
    RET
