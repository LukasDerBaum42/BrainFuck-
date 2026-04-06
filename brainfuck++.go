package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func termInit() {
	cmd := exec.Command("stty", "-icanon", "min", "1", "-echo")
	cmd.Stdin = os.Stdin
	cmd.Run()
}

func termRestore() {
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	cmd.Run()
}

func pollKey() byte {
	buf := make([]byte, 1)
	n, _ := os.Stdin.Read(buf)
	if n > 0 {
		return buf[0]
	}
	return 0
}

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
 *
 * technicly key words but not id ( ) { }
 */
type instruction struct {
	instr    uint8
	value    uint64
	f_name   string
	other    any
	switches map[byte]string
}

type function struct {
	name      string
	instructs []instruction
}

type frame struct {
	bytecode []instruction
	counter  uint64
}
type tape_t struct {
	name     string
	data     [0xFFFF + 1]byte
	position uint16
}

type loopInfo struct {
    startIdx uint64 // bytecode index of the matching [
    count    uint64 // iterations completed
}

var FUNCTS map[string]function
var TAPES map[string]tape_t

// callSite records a name reference (function call, tape switch, switch case)
// together with enough context to point at it in an error message.
type callSite struct {
	name       string
	nameOffset uint64  // byte offset of the first character of the name within code
	ctx        compCtx // compilation context at the call site
	code       string  // the local source slice that was being compiled
}

var called_functs []callSite
var called_tapes  []callSite

// keywordBytes lists every single-byte character that is a language keyword.
// The multi-byte keyword '§' is checked separately in validateName.
const keywordBytes = "+-<>[].,!?$*:(){}"

// validateName returns an error if name contains a keyword character or whitespace.
// nameStart is the byte offset of name[0] within code (used for error reporting).
func validateName(name string, nameStart uint64, ctx compCtx, code string) error {
	for i, ch := range name {
		off := nameStart + uint64(i)
		chStr := string(ch)
		if strings.ContainsRune(keywordBytes, ch) || chStr == "§" {
			return showError(ctx, code, off, "error",
				fmt.Sprintf("keyword character '%s' is not allowed in a name", chStr))
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			return showError(ctx, code, off, "error",
				"whitespace is not allowed in a name")
		}
	}
	return nil
}

// ── error formatting ──────────────────────────────────────────────────────────

// compCtx carries the information needed to render both local (within the
// current function body) and global (within the original source file) positions
// for every diagnostic.
type compCtx struct {
	globalSrc  string // full original source file, never changes
	baseOffset uint64 // byte offset of code[0] within globalSrc
	funcName   string // current function being compiled; "" = top level
}

type srcPos struct {
	line     int
	col      int
	lineText string
}

func offsetToPos(code string, offset uint64) srcPos {
	if offset > uint64(len(code)) {
		offset = uint64(len(code))
	}
	line, col, lineStart := 1, 1, 0
	for i := 0; i < int(offset); i++ {
		if code[i] == '\n' {
			line++
			col = 1
			lineStart = i + 1
		} else {
			col++
		}
	}
	lineEnd := lineStart
	for lineEnd < len(code) && code[lineEnd] != '\n' {
		lineEnd++
	}
	return srcPos{line: line, col: col, lineText: code[lineStart:lineEnd]}
}

// printDiagBlock renders one source-highlighted block with surrounding context.
func printDiagBlock(label, msg string, localPos srcPos, globalPos *srcPos, funcName string,
	displayCode string, displayOffset uint64) {

	const ctxBefore = 1
	const ctxAfter  = 1

	dispPos  := offsetToPos(displayCode, displayOffset)
	allLines := strings.Split(displayCode, "\n")

	first   := max(1, dispPos.line-ctxBefore)
	last    := min(len(allLines), dispPos.line+ctxAfter)
	gutterW := len(fmt.Sprintf("%d", last))
	pad     := strings.Repeat(" ", gutterW)
	caret   := strings.Repeat(" ", dispPos.col-1) + "^"

	fmt.Fprintf(os.Stderr, "%s: %s\n", label, msg)

	if globalPos != nil && funcName != "" {
		fmt.Fprintf(os.Stderr, "  %s    local  --> (in '%s') %d:%d\n",
			pad, funcName, localPos.line, localPos.col)
		fmt.Fprintf(os.Stderr, "  %s    global --> %d:%d\n",
			pad, globalPos.line, globalPos.col)
	} else {
		fmt.Fprintf(os.Stderr, "  %s --> %d:%d\n", pad, dispPos.line, dispPos.col)
	}

	fmt.Fprintf(os.Stderr, "  %s |\n", pad)
	for l := first; l <= last; l++ {
		lStr := fmt.Sprintf("%*d", gutterW, l)
		fmt.Fprintf(os.Stderr, "  %s | %s\n", lStr, allLines[l-1])
		if l == dispPos.line {
			fmt.Fprintf(os.Stderr, "  %s | %s\n", pad, caret)
		}
	}
	fmt.Fprintf(os.Stderr, "  %s |\n", pad)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// showError prints a formatted diagnostic and returns it as a Go error.
func showError(ctx compCtx, code string, offset uint64, label, msg string) error {
	localPos := offsetToPos(code, offset)

	var globalPos *srcPos
	if ctx.funcName != "" {
		gp := offsetToPos(ctx.globalSrc, ctx.baseOffset+offset)
		globalPos = &gp
	}

	displayCode   := code
	displayOffset := offset
	if ctx.funcName != "" && globalPos != nil {
		displayCode   = ctx.globalSrc
		displayOffset = ctx.baseOffset + offset
	}

	printDiagBlock(label, msg, localPos, globalPos, ctx.funcName, displayCode, displayOffset)

	gLine, gCol := localPos.line, localPos.col
	if globalPos != nil {
		gLine, gCol = globalPos.line, globalPos.col
	}
	return fmt.Errorf("%s at local %d:%d / global %d:%d", msg, localPos.line, localPos.col, gLine, gCol)
}

// showErrorWithNote prints a primary error block and a secondary note block.
func showErrorWithNote(
	ctx compCtx, code string,
	errOffset uint64, msg string,
	noteOffset uint64, note string,
) error {
	err := showError(ctx, code, errOffset, "error", msg)
	showError(ctx, code, noteOffset, "note", note) //nolint
	return err
}

// showContext prints a traceback frame for nested function compilation errors.
func showContext(ctx compCtx, name string) {
	if ctx.funcName != "" {
		fmt.Fprintf(os.Stderr, "  note: while compiling function '%s' (called from '%s')\n\n", name, ctx.funcName)
	} else {
		fmt.Fprintf(os.Stderr, "  note: while compiling function '%s'\n\n", name)
	}
}

// ── compiler ──────────────────────────────────────────────────────────────────

func comp_add_sub_move(counter uint64, code string, symbol byte) (uint64, instruction) {
	instuc := instruction{}
	switch symbol {
	case '+':
		instuc.instr = 0
	case '-':
		instuc.instr = 1
	case '<':
		instuc.instr = 2
	case '>':
		instuc.instr = 3
	}
	counter++
	if counter >= uint64(len(code)) {
		instuc.value = 1
		return counter, instuc
	}
	if code[counter] == symbol {
		var temp uint64 = 2
		counter++
		for counter < uint64(len(code)) && code[counter] == symbol {
			counter++
			temp++
		}
		instuc.value = temp
	} else if code[counter] >= '0' && code[counter] <= '9' {
		temp := string(code[counter])
		counter++
		for counter < uint64(len(code)) && code[counter] >= '0' && code[counter] <= '9' {
			temp += string(code[counter])
			counter++
		}
		out, _ := strconv.ParseUint(temp, 10, 64)
		instuc.value = out
	} else {
		instuc.value = 1
	}
	return counter, instuc
}

func comp_print(counter uint64, code string) (uint64, instruction) {
	instuc := instruction{instr: 6, value: 0}
	counter++
	if counter >= uint64(len(code)) {
		return counter, instuc
	}
	switch code[counter] {
	case '0':
		counter++
	case '1':
		instuc.value = 1
		counter++
	case '2':
		instuc.value = 2
		counter++
	case '3':
		instuc.value = 3
		counter++
	}
	return counter, instuc
}

func comp_func(counter uint64, code string, ctx compCtx) (uint64, instruction, error) {
	instr := instruction{instr: 8}
	counter++

	if counter >= uint64(len(code)) || code[counter] != '(' {
		return counter, instr, showError(ctx, code, counter-1, "error",
			"expected '(' after '!' to open function name")
	}
	counter++ // skip '('

	n_start := counter
	for counter < uint64(len(code)) && code[counter] != ')' && code[counter] != '{' {
		if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
			for counter < uint64(len(code)) && code[counter] != '\n' {
				counter++
			}
			continue
		}
		counter++
	}
	if counter >= uint64(len(code)) {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, "function name never closed with ')'",
			n_start, "function name started here",
		)
	}
	if code[counter] == '{' {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, "unexpected '{' inside function name — missing ')'",
			n_start, "function name started here",
		)
	}
	name := code[n_start:counter]
	if err := validateName(name, n_start, ctx, code); err != nil {
		return counter, instr, err
	}
	counter++ // skip ')'

	if counter >= uint64(len(code)) || code[counter] != '{' {
		return counter, instr, showError(ctx, code, counter, "error",
			fmt.Sprintf("expected '{' to open body of function '%s'", name))
	}
	bodyOpen := counter
	counter++ // skip '{'

	c_start := counter
	nested := 0
	for counter < uint64(len(code)) && (code[counter] != '}' || nested != 0) {
		if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
			for counter < uint64(len(code)) && code[counter] != '\n' {
				counter++
			}
			continue
		}
		if code[counter] == '{' {
			nested++
		} else if code[counter] == '}' {
			nested--
		}
		counter++
	}
	if counter >= uint64(len(code)) {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, fmt.Sprintf("body of function '%s' is never closed with '}'", name),
			bodyOpen, "body opened here",
		)
	}

	new_code := code[c_start:counter]
	counter++ // skip '}'

	childCtx := compCtx{
		globalSrc:  ctx.globalSrc,
		baseOffset: ctx.baseOffset + c_start,
		funcName:   name,
	}

	new_bytecode, err := make_bytecode(new_code, childCtx)
	if err != nil {
		showContext(ctx, name)
		return counter, instr, err
	}

	instr.f_name = name
	FUNCTS[name] = function{name: name, instructs: new_bytecode}
	return counter, instr, nil
}

func comp_load_func(counter uint64, code string, ctx compCtx) (uint64, instruction, error) {
	instr := instruction{instr: 9}
	counter++

	if counter >= uint64(len(code)) || code[counter] != '(' {
		return counter, instr, showError(ctx, code, counter-1, "error",
			"expected '(' after '?' to open function name")
	}
	counter++ // skip '('

	n_start := counter
	for counter < uint64(len(code)) && code[counter] != ')' {
		if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
			for counter < uint64(len(code)) && code[counter] != '\n' {
				counter++
			}
			continue
		}
		counter++
	}
	if counter >= uint64(len(code)) {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, "function call name never closed with ')'",
			n_start, "name started here",
		)
	}
	instr.f_name = code[n_start:counter]
	if err := validateName(instr.f_name, n_start, ctx, code); err != nil {
		return counter, instr, err
	}
	called_functs = append(called_functs, callSite{
		name:       instr.f_name,
		nameOffset: n_start,
		ctx:        ctx,
		code:       code,
	})

	counter++ // skip ')'
	return counter, instr, nil
}

// comp_case returns: new counter, case key, function name, name's start offset, error.
func comp_case(counter uint64, code string, ctx compCtx) (uint64, byte, string, uint64, error) {
	caseStart := counter
	counter++

	if counter >= uint64(len(code)) || code[counter] < '0' || code[counter] > '9' {
		return counter, 0, "", 0, showError(ctx, code, caseStart, "error",
			"expected a numeric value after '*' for case")
	}

	temp := string(code[counter])
	counter++
	for counter < uint64(len(code)) && code[counter] >= '0' && code[counter] <= '9' {
		temp += string(code[counter])
		counter++
	}
	out, _ := strconv.ParseUint(temp, 10, 64)
	if out > 255 {
		return counter, 0, "", 0, showError(ctx, code, caseStart+1, "error",
			fmt.Sprintf("case value %d overflows a byte (max 255)", out))
	}
	key_value := byte(out)

	if counter >= uint64(len(code)) || code[counter] != '(' {
		return counter, key_value, "", 0, showError(ctx, code, counter, "error",
			"expected '(' to open function name for case")
	}
	counter++ // skip '('

	n_start := counter
	for counter < uint64(len(code)) && code[counter] != ')' {
		if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
			for counter < uint64(len(code)) && code[counter] != '\n' {
				counter++
			}
			continue
		}
		counter++
	}
	if counter >= uint64(len(code)) {
		return counter, key_value, "", 0, showErrorWithNote(ctx, code,
			counter, "case function name never closed with ')'",
			n_start, "name started here",
		)
	}
	name := code[n_start:counter]
	if err := validateName(name, n_start, ctx, code); err != nil {
		return counter, key_value, "", 0, err
	}
	counter++ // skip ')'
	return counter, key_value, name, n_start, nil
}

func comp_switch(counter uint64, code string, ctx compCtx) (uint64, instruction, error) {
	instr := instruction{instr: 10, switches: make(map[byte]string)}
	counter++

	if counter < uint64(len(code)) && code[counter] == '(' {
		counter++ // skip '('
		n_start := counter
		for counter < uint64(len(code)) && code[counter] != ')' && code[counter] != '{' {
			if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
				for counter < uint64(len(code)) && code[counter] != '\n' {
					counter++
				}
				continue
			}
			counter++
		}
		if counter >= uint64(len(code)) {
			return counter, instr, showErrorWithNote(ctx, code,
				counter, "switch default name never closed with ')'",
				n_start, "name started here",
			)
		}
		if code[counter] == '{' {
			return counter, instr, showErrorWithNote(ctx, code,
				counter, "unexpected '{' inside switch default name — missing ')'",
				n_start, "name started here",
			)
		}
		instr.f_name = code[n_start:counter]
		if err := validateName(instr.f_name, n_start, ctx, code); err != nil {
			return counter, instr, err
		}
		called_functs = append(called_functs, callSite{
			name:       instr.f_name,
			nameOffset: n_start,
			ctx:        ctx,
			code:       code,
		})
		instr.value = 1
		counter++ // skip ')'
	}

	if counter >= uint64(len(code)) || code[counter] != '{' {
		return counter, instr, showError(ctx, code, counter, "error",
			"expected '{' to open switch body")
	}
	bodyOpen := counter
	counter++ // skip '{'

	for counter < uint64(len(code)) && code[counter] != '}' {
		if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
			for counter < uint64(len(code)) && code[counter] != '\n' {
				counter++
			}
			continue
		}
		if code[counter] == '*' {
			c, k, n, nOff, err := comp_case(counter, code, ctx)
			if err != nil {
				return c, instr, err
			}
			instr.switches[k] = n
			called_functs = append(called_functs, callSite{
				name:       n,
				nameOffset: nOff,
				ctx:        ctx,
				code:       code,
			})
			counter = c
		} else {
			counter++
		}
	}
	if counter >= uint64(len(code)) {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, "switch body never closed with '}'",
			bodyOpen, "body opened here",
		)
	}
	counter++ // skip '}'
	return counter, instr, nil
}

func comp_create_tape(counter uint64, code string, ctx compCtx) (uint64, instruction, error) {
	instr := instruction{instr: 12}
	counter++

	if counter >= uint64(len(code)) || code[counter] != '(' {
		return counter, instr, showError(ctx, code, counter-1, "error",
			"expected '(' after '§' to open tape name")
	}
	counter++ // skip '('

	n_start := counter
	for counter < uint64(len(code)) && code[counter] != ')' {
		if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
			for counter < uint64(len(code)) && code[counter] != '\n' {
				counter++
			}
			continue
		}
		counter++
	}
	if counter >= uint64(len(code)) {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, "tape name never closed with ')'",
			n_start, "name started here",
		)
	}
	instr.f_name = code[n_start:counter]
	if err := validateName(instr.f_name, n_start, ctx, code); err != nil {
		return counter, instr, err
	}
	if instr.f_name == "main" {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, "tape name cannot be 'main'",
			n_start, "name started here",
		)
	}

	TAPES[instr.f_name] = tape_t{name: instr.f_name}
	counter++ // skip ')'
	return counter, instr, nil
}

func comp_switch_tape(counter uint64, code string, ctx compCtx) (uint64, instruction, error) {
	instr := instruction{instr: 13}
	counter++

	if counter >= uint64(len(code)) || code[counter] != '(' {
		instr.value = 0
		return counter, instr, nil
	}
	counter++ // skip '('

	n_start := counter
	for counter < uint64(len(code)) && code[counter] != ')' {
		if counter+1 < uint64(len(code)) && code[counter] == '/' && code[counter+1] == '/' {
			for counter < uint64(len(code)) && code[counter] != '\n' {
				counter++
			}
			continue
		}
		counter++
	}
	if counter >= uint64(len(code)) {
		return counter, instr, showErrorWithNote(ctx, code,
			counter, "tape name never closed with ')'",
			n_start, "name started here",
		)
	}
	instr.f_name = code[n_start:counter]
	if err := validateName(instr.f_name, n_start, ctx, code); err != nil {
		return counter, instr, err
	}
	if instr.f_name == "main" {
		instr.value = 0
	} else {
		instr.value = 1
		called_tapes = append(called_tapes, callSite{
			name:       instr.f_name,
			nameOffset: n_start,
			ctx:        ctx,
			code:       code,
		})
	}

	counter++ // skip ')'
	return counter, instr, nil
}

func make_bytecode(code string, ctx compCtx) ([]instruction, error) {
	var bytecode []instruction
	var counter uint64

	type loopEntry struct {
		bcIdx     uint64
		srcOffset uint64
	}
	var loops []loopEntry
	code_len := uint64(len(code))

	for counter < code_len {
		key := code[counter]
		switch key {
		default:
			counter++
		case '/':
			counter++
			if counter < code_len && code[counter] == '/' {
				for counter < code_len && code[counter] != '\n' {
					counter++
				}
				continue
			}
		case '+', '-', '<', '>':
			var temp instruction
			counter, temp = comp_add_sub_move(counter, code, key)
			bytecode = append(bytecode, temp)
		case '[':
			loops = append(loops, loopEntry{uint64(len(bytecode)), counter})
			bytecode = append(bytecode, instruction{instr: 4, other: counter})
			counter++
		case ']':
			if len(loops) == 0 {
				return bytecode, showError(ctx, code, counter, "error",
					"unexpected ']' — no matching '['")
			}
			top := loops[len(loops)-1]
			loops = loops[:len(loops)-1]
			bytecode[top.bcIdx].value = uint64(len(bytecode))
			bytecode = append(bytecode, instruction{instr: 5, value: top.bcIdx})
			counter++
		case '.':
			var temp instruction
			counter, temp = comp_print(counter, code)
			bytecode = append(bytecode, temp)
		case ',':
			bytecode = append(bytecode, instruction{instr: 7})
			counter++
		case '!':
			var temp instruction
			var err error
			counter, temp, err = comp_func(counter, code, ctx)
			if err != nil {
				return bytecode, err
			}
			bytecode = append(bytecode, temp)
		case '?':
			var temp instruction
			var err error
			counter, temp, err = comp_load_func(counter, code, ctx)
			if err != nil {
				return bytecode, err
			}
			bytecode = append(bytecode, temp)
		case '$':
			var temp instruction
			var err error
			counter, temp, err = comp_switch(counter, code, ctx)
			if err != nil {
				return bytecode, err
			}
			bytecode = append(bytecode, temp)
		case '§':
			var temp instruction
			var err error
			counter, temp, err = comp_create_tape(counter, code, ctx)
			if err != nil {
				return bytecode, err
			}
			bytecode = append(bytecode, temp)
		case ':':
			var temp instruction
			var err error
			counter, temp, err = comp_switch_tape(counter, code, ctx)
			if err != nil {
				return bytecode, err
			}
			bytecode = append(bytecode, temp)
		}
	}

	if len(loops) > 0 {
		top := loops[len(loops)-1]
		return bytecode, showErrorWithNote(ctx, code,
			code_len, "reached end of input with unclosed '['",
			top.srcOffset, "'[' opened here",
		)
	}

	return bytecode, nil
}

// ── interpreter ───────────────────────────────────────────────────────────────

func interpret_bytecode(entry []instruction) {
	//var POINTER_POS uint16
	//var REGISTER []byte
	var tape *tape_t
	//var ON_TAPE bool = false
	tape_map := make(map[string]uint16)
	tapes_array := make([]tape_t, 0, len(TAPES))

	var temp uint16 = 0
	for _, elem := range TAPES {
		tapes_array = append(tapes_array, tape_t{name: elem.name})
		tape_map[elem.name] = temp
		temp++
	}
 	loop_stack := make([]loopInfo, 0)

	//REGISTER = make([]byte, 0xffff+1)
	
	tape = &tapes_array[tape_map["main"]]
	stack := []frame{{bytecode: entry, counter: 0}}

	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.counter >= uint64(len(top.bytecode)) {
			stack = stack[:len(stack)-1]
			continue
		}
		instr := top.bytecode[top.counter]

		depth := len(loop_stack)
		firstIter := true
		for _, li := range loop_stack {
		    if li.count > 0 {
		        firstIter = false
		        break
		    }
		}
		
		if *interpreter_debug {
		    if firstIter || *show_loop {
		        if depth > 0 && *show_loop {
		            fmt.Print("loop depth:", depth, " ")
		            fmt.Print("loop count:", loop_stack[depth-1].count, " ")
		        }
		        fmt.Print("counter:", top.counter, " ")
		        fmt.Print("stack len:", len(stack), " ")
		        fmt.Print("on tape:", tape.name, " ")
		        fmt.Print("tape position:", tape.position, " ")
		        print_inst(instr, false)
		    }
		}

		switch instr.instr {
		default:
			top.counter++
		case 0:
			tape.data[tape.position] += byte(instr.value)
			top.counter++
		case 1:
			tape.data[tape.position] -= byte(instr.value)
			top.counter++
		case 2:
			tape.position -= uint16(instr.value)
			top.counter++
		case 3:
			tape.position += uint16(instr.value)
			top.counter++
		case 4:
		    if tape.data[tape.position] == 0 {
		        if *interpreter_debug && (firstIter || *show_loop) {
		            fmt.Print("loop skipped\n")
		        }
		        top.counter = instr.value
		    } else {
		        if len(loop_stack) == 0 || loop_stack[len(loop_stack)-1].startIdx != top.counter {
		            loop_stack = append(loop_stack, loopInfo{startIdx: top.counter})
		        }
		    }
		    top.counter++
		case 5:
		    if tape.data[tape.position] != 0 {
		        if len(loop_stack) > 0 {
		            loop_stack[len(loop_stack)-1].count++
		        }
		        top.counter = instr.value
		    } else {
		        if len(loop_stack) > 0 {
		            info := loop_stack[len(loop_stack)-1]
		            loop_stack = loop_stack[:len(loop_stack)-1]
		            if *interpreter_debug && (firstIter || *show_loop) {
		                fmt.Print("loop done — depth:", len(loop_stack)+1,
		                    " iterations:", info.count+1, "\n")
		            }
		        }
		        top.counter++
		    }
		case 6:
			switch instr.value {
			case 0:
				fmt.Print(string(tape.data[tape.position]))
			case 1:
				fmt.Print(tape.data[tape.position])
			case 2:
				fmt.Printf("0x%02x", tape.data[tape.position])
			case 3:
				fmt.Printf("0b%08b", tape.data[tape.position])
			}
			top.counter++
		case 7:
			tape.data[tape.position] = pollKey()
			top.counter++
		case 9:
			n_func := FUNCTS[instr.f_name]
			top.counter++
			stack = append(stack, frame{bytecode: n_func.instructs, counter: 0})
		case 10:
			var key byte
			key = tape.data[tape.position]
			top.counter++
			if val, ok := instr.switches[key]; ok {
				stack = append(stack, frame{bytecode: FUNCTS[val].instructs, counter: 0})
			} else if instr.value == 1 {
				stack = append(stack, frame{bytecode: FUNCTS[instr.f_name].instructs, counter: 0})
			}
		case 13:
			if instr.value == 0 {
				tape = &tapes_array[tape_map["main"]]
			} else {
				tape = &tapes_array[tape_map[instr.f_name]]
			}
			top.counter++
		}
	}
}

// ── C backend ─────────────────────────────────────────────────────────────────

func compile_to_c(bytecode []instruction, out_name string) {
	var sb strings.Builder
	
	sb.WriteString("#include <stdio.h>\n")
	sb.WriteString("#include <stdint.h>\n")
	sb.WriteString("#include <termios.h>\n")
	sb.WriteString("#include <unistd.h>\n\n")

	//sb.WriteString("uint8_t reg_main[0xffff] = {0};\n")
	//sb.WriteString("uint16_t ptr_main = 0;\n\n")

	sb.WriteString("struct termios orig_term;\n\n")
	
	sb.WriteString("void term_init() {\n")
	sb.WriteString("  struct termios raw;\n")
	sb.WriteString("  tcgetattr(STDIN_FILENO, &orig_term);\n")
	sb.WriteString("  raw = orig_term;\n")
	sb.WriteString("  raw.c_lflag &= ~(ICANON | ECHO);\n")
	sb.WriteString("  raw.c_cc[VMIN] = 1;\n")
	sb.WriteString("  raw.c_cc[VTIME] = 0;\n")
	sb.WriteString("  tcsetattr(STDIN_FILENO, TCSANOW, &raw);\n")
	sb.WriteString("}\n\n")

	sb.WriteString("void term_restore() {\n")
	sb.WriteString("  tcsetattr(STDIN_FILENO, TCSAFLUSH, &orig_term);\n")
	sb.WriteString("}\n\n")
	
	for name := range TAPES {
		sb.WriteString(fmt.Sprintf("uint8_t tape_%s[0xffff] = {0};\n", name))
		sb.WriteString(fmt.Sprintf("uint16_t ptr_%s = 0;\n", name))
	}
	
	sb.WriteString("\n")
	
	sb.WriteString(fmt.Sprintf("uint8_t *reg = tape_main;\n"))
	sb.WriteString(fmt.Sprintf("uint16_t *ptr = &ptr_main;\n"))

	sb.WriteString("\n")
	
	for name := range FUNCTS {
		sb.WriteString(fmt.Sprintf("void f_%s();\n", name))
	}
	for name, fn := range FUNCTS {
		sb.WriteString(fmt.Sprintf("\nvoid f_%s() {\n", name))
		write_c_body(&sb, fn.instructs, "  ")
		sb.WriteString("}\n")
	}
	
	

	sb.WriteString("\nint main() {\n")
	sb.WriteString("  term_init();\n")
	sb.WriteString("  f_main();\n")
	sb.WriteString("  term_restore();\n")
	sb.WriteString("  printf(\"\\n\");\n")
	sb.WriteString("  return 0;\n}\n")

	os.WriteFile(out_name+".c", []byte(sb.String()), 0644)

	if *output != "" {
		if err := invoke_compiler(*output); err != nil {
			fmt.Fprintln(os.Stderr, "error: compilation failed:", err)
		}
	}
	if !*print_bycode {
		os.Remove(out_name + ".c")
	}
}

func write_c_body(sb *strings.Builder, bytecode []instruction, indent string) {
	for _, instr := range bytecode {
		switch instr.instr {
		case 0:
			sb.WriteString(fmt.Sprintf("%sreg[*ptr] += %d;\n", indent, instr.value))
		case 1:
			sb.WriteString(fmt.Sprintf("%sreg[*ptr] -= %d;\n", indent, instr.value))
		case 2:
			sb.WriteString(fmt.Sprintf("%s*ptr -= %d;\n", indent, instr.value))
		case 3:
			sb.WriteString(fmt.Sprintf("%s*ptr += %d;\n", indent, instr.value))
		case 4:
			sb.WriteString(fmt.Sprintf("%swhile (reg[*ptr]) {\n", indent))
		case 5:
			sb.WriteString(fmt.Sprintf("%s}\n", indent))
		case 6:
			switch instr.value {
			case 0:
				sb.WriteString(fmt.Sprintf("%sputchar(reg[*ptr]);\n", indent))
			case 1:
				sb.WriteString(fmt.Sprintf("%sprintf(\"%%d\", reg[*ptr]);\n", indent))
			case 2:
				sb.WriteString(fmt.Sprintf("%sprintf(\"02x%%02x\", reg[*ptr]);\n", indent))
			case 3:
				sb.WriteString(fmt.Sprintf("%sprintf(\"08b%%08b\", reg[*ptr]);\n", indent))
			}
		case 7:
			sb.WriteString(fmt.Sprintf("%sreg[*ptr] = getchar();\n", indent))
		case 9:
			sb.WriteString(fmt.Sprintf("%sf_%s();\n", indent, instr.f_name))
		case 10:
			sb.WriteString(fmt.Sprintf("%sswitch (reg[*ptr]) {\n", indent))
			for k, v := range instr.switches {
				sb.WriteString(fmt.Sprintf("%s  case %d: f_%s(); break;\n", indent, k, v))
			}
			if instr.value == 1 {
				sb.WriteString(fmt.Sprintf("%s  default: f_%s(); break;\n", indent, instr.f_name))
			}
			sb.WriteString(fmt.Sprintf("%s}\n", indent))
		case 13:
			if instr.value == 0 {
				sb.WriteString(fmt.Sprintf("%sreg = tape_main;\n", indent))
				sb.WriteString(fmt.Sprintf("%sptr = &ptr_main;\n", indent))
			} else {
				sb.WriteString(fmt.Sprintf("%sreg = tape_%s;\n", indent, instr.f_name))
				sb.WriteString(fmt.Sprintf("%sptr = &ptr_%s;\n", indent, instr.f_name))
			}
		}
	}
}

func invoke_compiler(out_name string) error {
	args := []string{"-O2", "-o", out_name, out_name + ".c"}
	if *compiler_args != "" {
		args = append(args, strings.Fields(*compiler_args)...)
	}
	cmd := exec.Command(*compiler, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ── debug printing ────────────────────────────────────────────────────────────

func print_bytecode(bytecode []instruction, is_in_func bool) {
	for _, ele := range bytecode {
		if is_in_func {
			fmt.Print("  ")
		}
		print_inst(ele, is_in_func)
	}
}

func print_inst(ele instruction, is_in_func bool) {
	switch ele.instr {
	case 0:
		fmt.Print("add ", ele.value, "\n")
	case 1:
		fmt.Print("sub ", ele.value, "\n")
	case 2:
		fmt.Print("move_left ", ele.value, "\n")
	case 3:
		fmt.Print("move_right ", ele.value, "\n")
	case 4:
		fmt.Print("start_loop ", ele.value, "\n")
	case 5:
		fmt.Print("end_loop ", ele.value, "\n")
	case 6:
		fmt.Print("print ", ele.value, "\n")
	case 7:
		fmt.Print("read ", ele.value, "\n")
	case 8:
		fmt.Print("create_func ", ele.f_name, "\n")
	case 9:
		fmt.Print("call_func ", ele.f_name, "\n")
	case 10:
		if ele.value == 1 {
			fmt.Print("switch ", ele.value, " ", ele.f_name, "\n")
		} else {
			fmt.Print("switch ", ele.value, "\n")
		}
		for k, v := range ele.switches {
			if is_in_func {
				fmt.Print("  ")
			}
			fmt.Print("  case ", k, " ", v, "\n")
		}
	case 11:
		fmt.Print("jump_to ", ele.value, "\n")
	case 12:
		fmt.Print("create_tape ", ele.f_name, "\n")
	case 13:
		if ele.value == 0 {
			fmt.Print("switch_tape ", ele.value, " main", "\n")
		} else {
			fmt.Print("switch_tape ", ele.value, " ", ele.f_name, "\n")
		}
	}
}

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
 