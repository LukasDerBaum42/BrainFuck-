package main

import (
	"fmt"
	"os"
	"strings"
)


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
