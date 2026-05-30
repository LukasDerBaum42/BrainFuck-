package main

import (
	"fmt"
)

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