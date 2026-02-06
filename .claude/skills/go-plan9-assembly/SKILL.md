---
name: go-plan9-assembly
description: Writes Go assembly (.s) using Go’s Plan 9–style syntax, with correct ABI0 stack layout, symbols, directives, and GC-safe patterns. Use when generating or reviewing Go asm, porting hot loops to asm, or fixing build/ABI/stack issues.
---

When writing Go’s Plan 9–style assembly, always follow these rules and structure.

## 0. Start by locking the target

Before writing instructions, state (in your own head, but reflect it in choices):

- **GOARCH/GOOS** target (e.g., amd64/linux, arm64/darwin)
- **Calling style**: default to **ABI0** (stack-based, stable) unless explicitly asked to use ABIInternal.
- Whether the function is a **leaf** (no calls) or **non-leaf** (calls other functions).

If unsure, assume:

- GOARCH=amd64
- ABI0
- Leaf function first, then extend.

## 1. Use the canonical Go asm file structure

A typical Go asm file should look like:

1. Header includes (when needed)

- `#include "textflag.h"` for flags like `NOSPLIT`
- Optionally include `go_asm.h` (generated) for offsets/constants

2. One or more `TEXT` functions

3. Optional `DATA`/`GLOBL` for constants (prefer pointer-free data)

Keep each function self-contained and comment the Go signature above it.

## 2. Always use the Go pseudo-registers correctly

You must respect these addressing rules:

### Globals: `SB`

- Global and function symbols are referenced as `name(SB)`.

### Arguments/results: `FP`

- **Always** use the named form: `arg+offset(FP)` and `ret+offset(FP)`.
  - Never write plain `0(FP)`; Go asm expects a name for tooling/vet.

### Locals: virtual `SP`

- Locals are addressed like `local-8(SP)` (negative offsets are common).
- Distinguish virtual `SP` addressing from any hardware `SP` register semantics.

### Branches: `PC`

- Use normal instruction mnemonics for control flow; keep labels clear.

## 3. Symbol spelling rules (the “weird dots”)

When referencing package symbols:

- Use `·` (U+00B7) instead of `.`
- Use `∕` (U+2215) instead of `/`

Prefer package-local symbols as `·Name(SB)` inside the package.

## 4. Define functions with correct `TEXT` + frame/arg sizes

Always write functions like:

- `TEXT ·Func(SB), FLAGS, $framesize-argsize`

Rules:

- The `$framesize-argsize` is **two numbers**, not subtraction.
- If the function can grow the stack (i.e., not `NOSPLIT`), `argsize` must be correct.
- End every `TEXT` with `RET` (or an unconditional jump). No fallthrough.

### Choose flags intentionally

Default choices:

- Use `NOSPLIT` for small leaf routines where safe.
- Avoid exotic flags unless you know why you need them.

## 5. Calling convention: make the stack layout explicit

For ABI0, treat args/results as laid out in the caller’s frame and accessed via `FP`:

- Load args from `a+0(FP)`, `b+8(FP)`, etc. (offsets depend on types/arch)
- Store results to `ret+X(FP)`.

Always ensure offsets match the Go declaration.

## 6. GC safety rules (do not guess)

Go’s runtime needs correct pointer maps. Follow these safe defaults:

### Safest patterns (preferred)

- **Leaf functions** with:
  - No calls
  - No heap pointers in locals
  - No pointer-typed results (or results written once at end)

### If you have a local frame, assume it contains NO pointers

- Keep local stack data pointer-free.
- If the function has locals and is non-leaf, add the proper no-pointer annotation pattern (e.g., `NO_LOCAL_POINTERS`) if required by your use case/toolchain.

### Avoid defining pointer-containing globals in asm

- Prefer constants without pointers.
- If you need data containing pointers, define it in Go, not in `.s`.

If you’re unsure whether something is GC-safe:

- Redesign the asm to avoid pointers and calls, or move logic back to Go.

## 7. Data definitions: only for pointer-free constants

When you must define data in asm, use:

- `DATA sym+off(SB)/width, $value`
- `GLOBL sym(SB), flags, $size`

Rules:

- Offsets must increase monotonically.
- Keep the data pointer-free unless you _absolutely_ know what you’re doing.
- Prefer read-only data when appropriate.

## 8. Architecture specifics: pick the right registers and widths

Always match:

- Register names and calling patterns for GOARCH (e.g., amd64 uses `AX`, `BX`, …; arm64 uses `R0`/`X0`-style depending on Go syntax)
- Instruction widths (`MOVB/MOVW/MOVL/MOVQ` etc.) and alignment rules
- Zero-extension/sign-extension behavior when loading smaller types

If you’re porting from Go code:

- Start from compiler output and mirror its patterns.

## 9. Build constraints and filenames

Use Go’s build selection mechanisms:

- Filename suffixes like `_amd64.s`, `_arm64.s`, `_linux_amd64.s`
- Or `//go:build` constraints (when appropriate for `.s` in your setup)

Never assume the asm is portable across architectures.

## 10. Verification checklist (mandatory)

After writing asm, include a quick “sanity pass” mentally and (when possible) recommend these checks:

1. **Assembly compiles**

- `go test` / `go test -c` for the package

2. **Offsets match**

- Args/results offsets align with the Go signature
- Frame/arg sizes in `TEXT` are correct

3. **Control flow ends**

- Every path returns via `RET` or jumps to a `RET`

4. **GC safety**

- No pointers in locals (unless deliberately handled)
- No unsafe pointer-containing asm globals

5. **Disassembly review**

- Use `go tool objdump` to confirm the intended instructions are emitted

## 11. Provide templates, not just prose

When generating code, prefer to output:

- The Go declaration (signature) in a `.go` file snippet
- The `.s` function with:
  - `#include "textflag.h"`
  - Clear comments for arg/result offsets
  - Minimal instruction set
  - `RET` at end

If asked to optimize, do it in steps:

1. Correctness-first asm
2. Then micro-optimizations (instruction selection, unrolling, avoiding stalls)
3. Then benchmark-driven tuning

## Common gotchas (always mention at least one)

Pick the most relevant one for the user’s task:

- Using `0(FP)` instead of `name+0(FP)` (Go requires named FP references)
- Mixing up `framesize-argsize` (it’s not subtraction)
- Forgetting to store the return value into `ret+off(FP)`
- Introducing pointers in locals and breaking GC assumptions
- Using the wrong symbol spelling (must use `·` and `∕` in symbol names)
- Writing non-leaf asm without handling split-stack / GC constraints correctly
