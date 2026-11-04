#include "go_asm.h"
#include "funcdata.h"

TEXT ·pool_acquire(SB), $16-12
    NO_LOCAL_POINTERS

    MOVQ  $const_initialBackoffValue, R9 // backoff
    JMP   loop

backoff:
    MOVQ   $const_maxBackoffValue, AX
    CMPQ   AX, R9 
    JCS    sleep

    IMULQ  $2, R9

sleep:
    PAUSE
    MOVQ   R9, 0(SP)
    MOVQ   R9, 8(SP)
    CALL   time·Sleep(SB)

    MOVQ   8(SP), R9

loop:
    MOVQ   x+0(FP), R8 // R8 - pointer to bitmap
    MOVQ   (R8), AX    // dereference the pointer (relaxed)
    TESTQ  AX,  AX     // AX - expected bitmap value
    JE     backoff     // if bitmap == 0: continue

    XORQ   BX, BX
    BSFQ   AX, BX      // BX - offset

    MOVQ   $1, DX
    MOVQ   BX, CX
    SHLQ   CX, DX      // (1 << offset)
    XORQ   AX, DX      // DX - new bitmap value
    
    LOCK;  CMPXCHGQ DX, 0(R8) 
    JNE    sleep

    MOVL   BX, ret+8(FP)
    RET

TEXT ·pool_release(SB), $0-12
    MOVQ x+0(FP), AX // AX - pointer to bitmap 

    MOVQ $1, DX
    MOVQ x+8(FP), CX // index (offset)
    SHLQ CX, DX

    LOCK; ORQ  DX, 0(AX)

    RET
