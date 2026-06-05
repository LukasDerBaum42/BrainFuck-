package main
 
import (
	"fmt"
	"os"
)

// ── interpreter ───────────────────────────────────────────────────────────────

// transferArgs pops args from caller's stack onto callee's stack,
// verifying sizes against the declared signature.
// First arg slot matches the top of caller stack.
func transferArgs(callerStack *bfStack, calleeStack *bfStack, slots []sigSlot, funcName string) bool {
	for i, slot := range slots {
		entry, underflow := callerStack.pop()
		if underflow {
			fmt.Fprintf(os.Stderr,
				"runtime error: function '%s' arg %d: stack underflow during argument transfer\n",
				funcName, i+1)
			return false
		}
		if !slot.wildcard && entry.size != slot.exactSize {
			fmt.Fprintf(os.Stderr,
				"runtime error: function '%s' arg %d: expected %d-byte element, got %d-byte element\n",
				funcName, i+1, slot.exactSize, entry.size)
			return false
		}
		if !calleeStack.push(entry) {
			fmt.Fprintf(os.Stderr,
				"runtime error: function '%s': callee stack overflow during argument transfer\n",
				funcName)
			return false
		}
	}
	return true
}

// transferReturns pops return values from callee's stack onto caller's stack.
// The first listed return slot is the bottom of the values to transfer
// (so the last listed return ends up on top of caller stack).
func transferReturns(calleeStack *bfStack, callerStack *bfStack, slots []sigSlot, funcName string) bool {
	if len(slots) == 0 {
		return true
	}
	// collect from callee (top-first) then reverse-push onto caller
	// so that caller ends up with first-return at bottom, last-return on top
	collected := make([]stackEntry, len(slots))
	for i, slot := range slots {
		entry, underflow := calleeStack.pop()
		if underflow {
			fmt.Fprintf(os.Stderr,
				"runtime error: function '%s' return %d: stack underflow during return transfer\n",
				funcName, i+1)
			return false
		}
		if !slot.wildcard && entry.size != slot.exactSize {
			fmt.Fprintf(os.Stderr,
				"runtime error: function '%s' return %d: expected %d-byte element, got %d-byte element\n",
				funcName, i+1, slot.exactSize, entry.size)
			return false
		}
		collected[i] = entry
	}
	// push in reverse so first return ends up deepest, last return on top
	for i := len(collected) - 1; i >= 0; i-- {
		if !callerStack.push(collected[i]) {
			fmt.Fprintf(os.Stderr,
				"runtime error: function '%s': caller stack overflow during return transfer\n",
				funcName)
			return false
		}
	}
	return true
}

// interpFrame extends the execution frame with a value stack pointer.
type interpFrame struct {
	bytecode []instruction
	counter  uint64
	stack    *bfStack // nil for macro frames; inherit caller's via currentStack()
	funcName string   // non-empty for function-mode frames; used for return transfer
}

func interpret_bytecode(entry []instruction) {
	tape_map := make(map[string]uint16)
	tapes_array := make([]tape_t, 0, len(TAPES))

	var temp uint16 = 0
	for _, elem := range TAPES {
		tapes_array = append(tapes_array, tape_t{name: elem.name})
		tape_map[elem.name] = temp
		temp++
	}
	loop_stack := make([]loopInfo, 0)

	tape := &tapes_array[tape_map["main"]]

	// root stack for main
	rootStack := &bfStack{}

	stack := []interpFrame{{bytecode: entry, counter: 0, stack: rootStack}}

	// currentStack returns the active value stack for the top frame,
	// walking up through macro frames if needed.
	currentStack := func() *bfStack {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].stack != nil {
				return stack[i].stack
			}
		}
		return rootStack
	}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.counter >= uint64(len(top.bytecode)) {
			// frame ending — if it's a function frame, handle returns
			// (return transfer is done at call site after frame pops, see below)
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
				fmt.Printf("%02x", tape.data[tape.position])
			case 3:
				fmt.Printf("%08b", tape.data[tape.position])
			}
			top.counter++
		case 7:
			tape.data[tape.position] = pollKey()
			top.counter++
		case 9:
			fn := FUNCTS[instr.f_name]
			top.counter++

			if fn.isMacro {
				// macro: no stack transfer, just push a frame with nil stack
				// so it inherits the caller's stack via currentStack()
				stack = append(stack, interpFrame{
					bytecode: fn.instructs,
					counter:  0,
					stack:    nil,
				})
			} else {
				// function mode: transfer args
				callerVS := currentStack()
				calleeVS := &bfStack{}

				if !transferArgs(callerVS, calleeVS, fn.args, fn.name) {
					os.Exit(1)
				}

				// We need to handle returns after the callee finishes.
				// Strategy: push a sentinel "return frame" after the callee
				// by using a closure stored in a wrapper — but since Go
				// doesn't allow that easily in a slice loop, we use a small
				// trick: push the callee frame, and record a pending return
				// by storing the callee stack pointer on the frame itself.
				// After the frame pops we handle it in the loop below via
				// a deferred-returns mechanism.
				//
				// Simplest approach: push callee frame with its own stack.
				// After it naturally exits (counter >= len), we detect the
				// non-nil stack and do return transfer before fully popping.
				stack = append(stack, interpFrame{
					bytecode: fn.instructs,
					counter:  0,
					stack:    calleeVS,
				})

			}
		case 10:
			key := tape.data[tape.position]
			top.counter++
			var targetName string
			if val, ok := instr.switches[key]; ok {
				targetName = val
			} else if instr.value == 1 {
				targetName = instr.f_name
			}
			if targetName != "" {
				fn := FUNCTS[targetName]
				if fn.isMacro {
					stack = append(stack, interpFrame{
						bytecode: fn.instructs,
						counter:  0,
						stack:    nil,
					})
				} else {
					callerVS := currentStack()
					calleeVS := &bfStack{}
					if !transferArgs(callerVS, calleeVS, fn.args, fn.name) {
						os.Exit(1)
					}
					stack = append(stack, interpFrame{
						bytecode: fn.instructs,
						counter:  0,
						stack:    calleeVS,
						funcName: fn.name,
					})
				}
			}
		case 13:
			if instr.value == 0 {
				tape = &tapes_array[tape_map["main"]]
			} else {
				tape = &tapes_array[tape_map[instr.f_name]]
			}
			top.counter++

		// ── push (copy) ^^
		case 14:
			n := uint8(instr.value)
			var entry stackEntry
			entry.size = n
			for i := uint8(0); i < n; i++ {
				entry.data[i] = tape.data[tape.position+uint16(i)]
			}
			vs := currentStack()
			if !vs.push(entry) {
				fmt.Fprintln(os.Stderr, "runtime error: value stack overflow on push")
				os.Exit(1)
			}
			top.counter++

		// ── push (move) ^^^
		case 15:
			n := uint8(instr.value)
			var entry stackEntry
			entry.size = n
			for i := uint8(0); i < n; i++ {
				entry.data[i] = tape.data[tape.position+uint16(i)]
				tape.data[tape.position+uint16(i)] = 0
			}
			vs := currentStack()
			if !vs.push(entry) {
				fmt.Fprintln(os.Stderr, "runtime error: value stack overflow on move-push")
				os.Exit(1)
			}
			top.counter++

		// ── pop ──────────────────────────────────────────────────────────
		case 16:
			vs := currentStack()
			entry, underflow := vs.pop()
			if underflow {
				// underflow: write single zero byte, don't abort
				tape.data[tape.position] = 0
			} else {
				for i := uint8(0); i < entry.size; i++ {
					tape.data[tape.position+uint16(i)] = entry.data[i]
				}
			}
			top.counter++
		}

		// ── return transfer: detect when a function frame just finished ───
		// Re-check top of stack: if the frame we just processed has now
		// exhausted its bytecode AND has an own stack (non-macro function),
		// do return transfer now before the next iteration pops it.
		if len(stack) > 0 {
			finishing := &stack[len(stack)-1]
			if finishing.counter >= uint64(len(finishing.bytecode)) &&
				finishing.stack != nil && finishing.funcName != "" {

				fn := FUNCTS[finishing.funcName]
				if fn.returns != nil {
					// find caller stack: second-to-top frame's effective stack
					var callerVS *bfStack
					if len(stack) >= 2 {
						for i := len(stack) - 2; i >= 0; i-- {
							if stack[i].stack != nil {
								callerVS = stack[i].stack
								break
							}
						}
					}
					if callerVS == nil {
						callerVS = rootStack
					}
					if !transferReturns(finishing.stack, callerVS, fn.returns, fn.name) {
						os.Exit(1)
					}
				}
				// clear funcName so we don't double-transfer if loop hits again
				finishing.funcName = ""
			}
		}
	}
}
