# BrainFuck++ Language Reference

**bf++** is an esoteric programming language and its compiler/interpreter, written in Go by LukasDerBaum. It is a superset of BrainFuck that extends the original with functions, named tapes, a per-function stack, imports, and a dispatch (switch) construct, while deliberately staying minimal and painful to work with.

The compiler can either interpret bf++ programs directly or cross-compile them to C and produce native executable binaries.


## Programs

Every program must define a function called `main`; execution starts there. All code outside a function definition is silently ignored.


## Memory Model

### The tape

bf++ gives you a flat array of **65 536 bytes** (the *tape*), addressed by a movable pointer. The tape wraps — its behaviour at the edges follows standard BrainFuck conventions.

### Named tapes

Any number of additional tapes of the same size (65 536 bytes) can be declared with `§`. Each has its own independent pointer. At any moment exactly one tape is *active*; all arithmetic, movement, and I/O operations act on the active tape.

### Per-function stack

Every function has its own **stack** that can hold up to **256 entries**. Each entry holds up to **255 bytes**. The stack is used to pass arguments to and receive return values from other functions.


## Instruction Reference

### Arithmetic and pointer movement

These instructions always operate on the currently active tape.

| Syntax | Effect |
| - | - |
| `+` | Add 1 to the current cell |
| `+N` or `+++` | Add N to the current cell (literal number *or* repeated symbol) |
| `-` | Subtract 1 from the current cell |
| `-N` or `---` | Subtract N from the current cell |
| `\>` | Move the pointer 1 cell to the right |
| `\>N` or `\>\>\>` | Move the pointer N cells to the right |
| `\<` | Move the pointer 1 cell to the left |
| `\<N` or `\<\<\<` | Move the pointer N cells to the left |


N may be written as a decimal integer literal (e.g. `+10`, `\>5`) or as the symbol repeated (e.g. `+++`, `\>\>\>`). Both forms are equivalent.

### Control flow

| Syntax | Effect |
| - | - |
| `\[` | If the current cell is **zero**, jump past the matching `\]` |
| `\]` | If the current cell is **non-zero**, jump back to the matching `\[` |


These are standard BrainFuck loops.

### I/O

| Syntax | Output format |
| - | - |
| `.` or `.0` | ASCII character |
| `.1` | Decimal integer |
| `.2` | Hexadecimal (2 digits, e.g. `1f`) |
| `.3` | Binary (8 digits, e.g. `01000001`) |
| `,` | Read one keypress (raw, unbuffered) into the current cell |


### Stack operations

| Syntax | Effect |
| - | - |
| `^N` | **Copy** N bytes starting at the current cell and push them onto the stack as one entry |
| `^^N` | **Move** N bytes (copy then zero the source cells) onto the stack as one entry |
| `v` | **Pop** the top stack entry and write it back to the tape starting at the current cell |


N must be 1–255. The `^` and `^^` forms without a number default to `^1`.


## Functions and Macros

### Definition syntax

```
!(name)\{...\}
```

Define a **macro**. The body is expanded inline at every call site; macros share the caller's stack.

```
!(name)()\{...\}
```

Define a **function** with no arguments and no return values.

```
!(name)(arg1, arg2, ...)\{...\}
```

Define a function with arguments. Arguments are popped from the **caller's** stack and pushed onto the **function's** stack in **reverse order** — the first argument listed, which is at the top of the caller’s stack, ends up on the bottom of the function's stack, last listed argument ends up on top of the function's stack. 

```
!(name)()(ret1, ret2, ...)\{...\}
```

Define a function with no arguments but with return values. Return values are popped from the **function's** stack and pushed onto the **caller's** stack in **reverse order** when the function returns.

```
!(name)(arg1, arg2, ...)(ret1, ret2, ...)\{...\}
```

Define a function with both arguments and return values.

### Argument / return slot types

Inside a `(...)` signature, each slot is one of:

| Slot | Meaning |
| - | - |
| `^N` | Exactly N bytes (1–255) |
| `n` | Any number of bytes (wildcard) |


Slots are separated by commas.

### Calling

```
?(name)
```

Call a previously defined function or macro. For functions, the required arguments must already be on the stack before the call.

### Execution rules

- Functions have their **own stack**, independent of the caller.

- Macros are **inlined** and share the caller's stack.

- Every program must have a `main` function; it is the entry point.


## Switch / Dispatch

```
$(default)\{ \*V(fn)  \*V(fn) ... \}
```

Reads the **current cell**, then calls the function whose case value matches.

- `\*V(fn)` — when the cell equals byte value V (0–255), call function `fn`.

- `(default)` — optional; names the function to call when no case matches. Omit to do nothing on a miss.

- Cases may appear in any order inside the body.

**Example:**

```
$(handle\_unknown)\{  
    \*65(handle\_A)  
    \*66(handle\_B)  
    \*10(handle\_newline)  
\}
```


## Named Tapes

| Syntax | Effect |
| - | - |
| `§(name)` | Declare a new tape called `name` |
| `:(name)` | Switch to tape `name`; all subsequent `+ - \< \> . ,` operations use it |
| `:` | Switch back to the main tape |


The name `main` is reserved and cannot be used for a tape.


## Imports

```
@(name)
```

or equivalently:

```
@0(name)
```

Include and compile the file `name.bfpp` from the current directory. The file's top-level function definitions become available in the importing program. Circular or duplicate imports are an error.

> **Note:** Import types `@1`, `@2`, `@3` are reserved for future use and are not currently supported.


## Comments

```
// this is a comment
```

Everything from `//` to the end of the line is ignored. Comments are valid anywhere in the source, including inside function bodies and switch blocks.


## Names

Function, macro, and tape names may contain any characters **except**:

- Language symbols: `+ - \< \> \[ \] . , ! ? $ \* : ( ) ^ @ \{ \} § /`

- Whitespace

The name `main` is reserved for the entry-point function. The name `main` is also reserved and cannot be used as a tape name.


## Quick Reference Card

```
+\[N\]  -\[N\]   arithmetic on current cell  
\>\[N\]  \<\[N\]   move pointer  
\[ \]           loop while cell ≠ 0  
.  .1  .2  .3  print (ASCII / decimal / hex / binary)  
,             read keypress  
^\[N\]  ^^\[N\]  push copy / move N bytes onto stack  
v             pop top of stack to current cell  
  
!(name)\{…\}                      define macro (inlined)  
!(name)(args)(rets)\{…\}          define function  
?(name)                         call function or macro  
  
$(default)\{ \*V(fn) … \}          dispatch on current cell  
  
§(name)   declare named tape  
:(name)   activate named tape  
:         return to main tape  
  
@(name)   import name.bfpp  
  
// …      line comment
```


## Example

```
@(utils)       // import utils.bfpp  
§(scratch)     // declare a named tape  
  
!(greet)()\{  
    \[-\] +72 .  // H  
    \[-\] +101 . // e  
    \[-\] +108 . // l  
    \[-\] +108 . // l  
    \[-\] +111 . // o  
    \[-\] +10 .  // newline  
\}  
  
!(main)\{  
    ?(greet)  
  
    :(scratch)   // work on the scratch tape  
    +5  
    \[  
        \< +7 \> -  
    \]  
    :            // back to main tape  
\}
```

