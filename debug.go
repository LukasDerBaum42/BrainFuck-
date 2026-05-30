package main

import (
	"fmt"
)


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