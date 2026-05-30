package main

import (
	"flag"
	"fmt"
	"os"
	// "os/exec"
	// "strconv"
	// "strings"
	// "runtime"
)



/*  0 (+) = add
 *  1 (-) = sub
 *  2 (<) = move left
 *  3 (>) = move right
 *  4 ([) = start loop
 *  5 (]) = end loop
 *  6 (.) = print |  0 / none = ascii , 1 = int , 2 = hex , 3 = bin
 *  7 (,) = read
 *  8 (!) = create func
 *  9 (?) = call func
 * 10 ($) = switch
 * 11 (*) = case
 * 12 (§) = create tape
 * 13 (:) = switch tape
 * 14 (^) = push stack
 * 15 (v) = pop stack
 * 16 (@) = extern | 0 / none = other bf++ program , 1 = c lib file , 2 = c header file , 3 = c function
 * 
 * technicly key words but not id ( ) { }
 */

 

// ── entry point ───────────────────────────────────────────────────────────────
 
var print_bycode       = flag.Bool("d", false, "print bytecode")
var interpret          = flag.Bool("i", false, "interpret the file")
var interpreter_debug  = flag.Bool("id", false, "interpreter debug info (only when interpreting)")
var show_loop          = flag.Bool("l", false, "show loop count (only when the -id flag is given)")
var output             = flag.String("o", "", "compile to output file")
var compiler           = flag.String("c", "gcc", "c compiler to use")
var compiler_args      = flag.String("cargs", "", "extra compiler arguments")
 
func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [flags] <file>
 
At least one of -i, -o, or -d must be given.
 
Flags:
`, os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `

			
Language info:
  The name of this language is BrainFuck++.
  It's a superset of BrainFuck, Developed by LukasDerBaum.
  It extends BrainFuck functionality with features that make it more useful
  while still trying to be minimal and and a pain to work with.
  
  The name of this compiler / interpreter is bf++
  bf++ is just a short for BrainFuck++
  and is written in Go
  
  This compiler / interpreter well alows you to compile your brainfuck++ code into executable binaries or interpret it directly.
  
  Importent: AI's like claude where used to generate this documentation (exept for this language Info) and parts of the code.

Language reference:
  Every program must define a function called 'main'; execution starts there.
  All code outside a function definition is ignored by the interpreter/compiler.
 
  Memory
    The interpreter has a flat array of 65 536 bytes or 64 kb (the "register") addressed
    by a movable pointer, plus any number of named "tapes" the same size.
 
  Arithmetic / movement  (on whichever memory is active)
    +[N]   add N to the current cell (N defaults to 1; repeat '+' or write a
           literal number, e.g. +5 or +++)
    -[N]   subtract N from the current cell
    >[N]   move the pointer N cells to the right
    <[N]   move the pointer N cells to the left
 
  Control flow
    [      jump past the matching ']' if the current cell is zero
    ]      jump back to the matching '[' if the current cell is non-zero
 
  I/O
    .      print the current cell as an ASCII character
    .1     print as a decimal integer
    .2     print as hex  (2x..)
    .3     print as binary (8b........)
    ,      read one keypress into the current cell (raw, unbuffered)
 
  Functions
    !(name){ ... }   define a function; the body is compiled recursively
    ?(name)          call a previously defined function
 
  Switch / dispatch  (reads current cell, calls the matching function)
    $(default){ *V(fn)  *V(fn) ... }
       Each *V(fn) maps byte value V to function fn.
       The optional (default) name is called when no case matches.
       Omit (default) to do nothing on a miss.
 
  Named tapes  (independent arrays with their own pointer)
    §(name)   declare a new tape called 'name'  (cannot be 'main')
    :(name)   switch to tape 'name' — all subsequent +/-/</>/./, ops act on it
    :         (no argument) switch back to the main register
 
  Comments
    // ...   everything after // until end of line is ignored
 
  Names
    Function and tape names may contain any characters except language keywords
    (+ - < > [ ] . , ! ? $ * : ( ) { } § /) and whitespace.
`)
	}
 
	flag.Parse()
 
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: no input file given")
		flag.Usage()
		return
	}
	if !*interpret && *output == "" && !*print_bycode {
		fmt.Fprintln(os.Stderr, "error: nothing to do — use at least one of -i, -o, -d")
		flag.Usage()
		return
	}
 
	content, err := os.ReadFile(args[len(args)-1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not read file:", err)
		return
	}
 
	src := string(content)
	FUNCTS = make(map[string]function)
	TAPES = make(map[string]tape_t)
	TAPES["main"] = tape_t{name: "main", data: [0xFFFF + 1]byte{}, position: 0}
 
	called_functs = make([]callSite, 0)
	called_tapes  = make([]callSite, 0)
 
	rootCtx := compCtx{globalSrc: src, baseOffset: 0, funcName: ""}
	bytecode, err := make_bytecode(src, rootCtx)
	if err != nil {
		os.Exit(1)
	}
 
	// Verify every referenced function and tape was actually defined,
	// and point at the exact call site if not.
	not_found := false
	for _, cs := range called_functs {
		if _, ok := FUNCTS[cs.name]; !ok {
			showError(cs.ctx, cs.code, cs.nameOffset, "error",
				fmt.Sprintf("function '%s' is called here but never defined", cs.name))
			not_found = true
		}
	}
	for _, cs := range called_tapes {
		if _, ok := TAPES[cs.name]; !ok {
			showError(cs.ctx, cs.code, cs.nameOffset, "error",
				fmt.Sprintf("tape '%s' is switched to here but never created", cs.name))
			not_found = true
		}
	}
 
	if not_found {
		os.Exit(1)
	}
 
	if *print_bycode {
		print_bytecode(bytecode, false)
		for i, ele := range FUNCTS {
			fmt.Println()
			fmt.Printf(":%s\n", i)
			print_bytecode(ele.instructs, true)
		}
	}
 
	if *interpret {
		start_func, ok := FUNCTS["main"]
		if !ok {
			fmt.Fprintln(os.Stderr, "error: no 'main' function defined")
			os.Exit(1)
		}
		termInit()
		defer termRestore()
		interpret_bytecode(start_func.instructs)
		fmt.Println()
	}
 
	if *output != "" {
		compile_to_c(bytecode, *output)
	}
}
 