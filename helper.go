package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	// "runtime"
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
