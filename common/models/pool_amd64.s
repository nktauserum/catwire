TEXT ·pool_acquire(SB), $0-12
    MOVQ x+0(FP), R8 // R8 - pointer to bitmap 

loop:
    PAUSE
    MOVQ  (R8), AX   // dereference the pointer (relaxed)
    TESTQ AX,  AX   // AX - expected bitmap value
    JE    loop      // if bitmap == 0: continue

    XORQ   BX, BX
    TZCNTQ AX, BX   // BX - offset

    MOVQ   $1, DX
    MOVQ   BX, CX
    SHLQ   CX, DX   // (1 << offset)
    XORQ   AX, DX   // DX - new bitmap value
    
    LOCK
    CMPXCHGQ DX, 0(R8) 

    JNE    loop

    MOVL   BX, ret+8(FP)
    RET
