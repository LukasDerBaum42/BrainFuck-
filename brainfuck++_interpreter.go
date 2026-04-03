package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
 *  4 ([) = strat loop
 *  5 (]) = end loop
 *  6 (.) = print |  0 / none = ascii , 1 = int , 2 = hex , 3 = bin
 *  7 (,) = read
 *  8 (!) = creat func
 *  9 (?) = call func
 * 10 ($) = switch
 * 11 (*) = case 
 */
type instruction struct {
	instr uint8
	value uint64
	f_name string
	other any
	switches map[byte]string
}

type function struct {
	name string
	instructs []instruction
}


var KILL bool = false
var FUNCTS map[string]function




func comp_add_sub_move(counter uint64, code string, symbol byte) (uint64, instruction) {
	instuc := instruction{instr: 0}
	switch symbol {
	default:
		instuc.instr = 0
	case '+':
		instuc.instr = 0
	case '-':
		instuc.instr = 1
	case '<':
		instuc.instr = 2
	case '>':
		instuc.instr = 3
	}

	counter += 1

	// Bounds check before first access
	if counter >= uint64(len(code)) {
		instuc.value = 1
		return counter, instuc
	}

	if code[counter] == symbol {
		var temp uint64 = 2
		counter += 1
		// Check BEFORE accessing code[counter] in loop
		for counter < uint64(len(code)) && code[counter] == symbol {
			counter += 1
			temp += 1
		}
		instuc.value = temp

	} else if code[counter] >= '0' && code[counter] <= '9' {
		var temp string = string(code[counter])
		counter += 1
		// Check BEFORE accessing code[counter] in loop
		for counter < uint64(len(code)) && code[counter] >= '0' && code[counter] <= '9' {
			temp += string(code[counter])
			counter += 1
		}
		var out uint64
		out, _ = strconv.ParseUint(temp, 10, 64)
		instuc.value = out

	} else {
		instuc.value = 1
	}

	return counter, instuc
}

func comp_print(counter uint64, code string) (uint64, instruction) {
	instuc := instruction{instr: 6, value: 0}
	counter += 1
	if counter >= uint64(len(code)) {
		return counter, instuc
	}
	key :=code[counter]
	switch key{
		case '0':
		counter += 1
		case '1':
		instuc.value = 1
		counter += 1
		case '2':
		instuc.value = 2
		counter += 1
		case '3':
		instuc.value = 3
		counter += 1
	}
	return counter, instuc
}


func comp_func(counter uint64, code string) (uint64, instruction) {
	instr := instruction{instr: 8}
	var new_code string
	var name string
	counter += 1
	if counter < uint64(len(code)) && code[counter] == '(' {
		counter += 1
		n_start := counter
		for counter < uint64(len(code)) && code[counter] != ')' && code[counter] != '{' {
			counter += 1
		}
		if counter >= uint64(len(code)) || code[counter] == '{' {
			KILL = true
			fmt.Println("function name not closed:",n_start)
			return counter,instr
		}
		n_end := counter
		name = code[n_start:n_end]
		counter += 1
		if counter < uint64(len(code)) && code[counter] != '{' {
			KILL = true
			fmt.Println("function dosen’t have a body:",counter)
			return counter,instr
		}
		counter += 1
		c_start := counter
		for counter < uint64(len(code)) && code[counter] != '}' {
			counter += 1
		}
		if counter >= uint64(len(code)) {
			KILL = true
			fmt.Println("function not closed:",n_start-1)
			return counter,instr
		}
		c_end := counter
		new_code = code[c_start:c_end]
		//fmt.Print(new_code)
		new_bycode := make_bytecode(new_code)
		if KILL {
			return counter,instr
		}
		counter += 1
		
		instr.f_name = name
		new_func := function{name: name,instructs: new_bycode}
		FUNCTS[name] = new_func
		
		return counter,instr
	} else {
		KILL = true
		fmt.Println("function doesn’t have a name:",counter-1)
		return counter,instr
	}
}

func comp_load_func(counter uint64, code string) (uint64, instruction) {
	instr := instruction{instr: 9}
	var name string
	counter += 1
	if code[counter] == '(' {
		counter += 1
		n_start := counter
		for counter < uint64(len(code)) && code[counter] != ')' {
			counter += 1
		}
		if counter >= uint64(len(code)) {
			KILL = true
			fmt.Println("function name not closed:",n_start)
			return counter,instr
		}
		n_end := counter
		name = code[n_start:n_end]
		instr.f_name = name
		counter += 1
		return counter,instr
	} else {
		KILL = true
		fmt.Println("function doesn’t have a name:",counter-1)
		return counter,instr
	}
}

func comp_case(counter uint64, code string) (uint64,byte,string) {
	var name string
	counter += 1
	if counter < uint64(len(code)) && code[counter] >= '0' && code[counter] <= '9' {
		var temp string = string(code[counter])
		counter += 1
		// Check BEFORE accessing code[counter] in loop
		for counter < uint64(len(code)) && code[counter] >= '0' && code[counter] <= '9' {
			temp += string(code[counter])
			counter += 1
		}
		var out uint64
		out, _ = strconv.ParseUint(temp, 10, 64)
		key_value := byte(out)
		//instuc.value = out
		if code[counter] != '(' {
			KILL = true
			fmt.Println("case dosen’t have a name:",counter)
			return counter,key_value,""
		}
		counter += 1
		n_start := counter
		for counter < uint64(len(code)) && code[counter] != ')' {
			counter += 1
		}
		if counter >= uint64(len(code)) {
			KILL = true
			fmt.Println("case name not closed:",n_start)
			return counter,key_value,""
		}
		n_end := counter
		name = code[n_start:n_end]
		return counter,key_value,name
	} else {
		KILL = true
		fmt.Println("case doesn’t have a value:",counter-1)
		return counter, 0 ,""
	}
}


func comp_switch(counter uint64, code string) (uint64, instruction) {
	instr := instruction{instr: 10,value: 0,switches: make(map[byte]string)}
	//var new_code string
	var name string
	counter += 1
	if counter < uint64(len(code)) && code[counter] == '(' {
		counter += 1
		n_start := counter
		for counter < uint64(len(code)) && code[counter] != ')' && code[counter] != '{' {
			counter += 1
		}
		if counter >= uint64(len(code)) || code[counter] == '{' {
			KILL = true
			fmt.Println("switch default not closed:",n_start)
			return counter,instr
		}
		n_end := counter
		name = code[n_start:n_end]
		instr.f_name = name
		instr.value = 1
	}
	if counter < uint64(len(code)) && code[counter] == '{' {
		counter += 1
		c_start := counter
		for counter < uint64(len(code)) && code[counter] != '}' {
			if code[counter] == '*' {
				c, k, n := comp_case(counter,code)
				instr.switches[k] = n
				counter = c
			}
			counter += 1
		}
		if counter >= uint64(len(code)) {
			KILL = true
			fmt.Println("switch not closed:",c_start)
			return counter,instr
		}
		//c_end := counter
		//new_code = code[c_start:c_end]
		//fmt.Print(new_code)
		//new_bycode := make_bytecode(new_code)
		if KILL {
			return counter,instr
		}
		counter += 1
		
		//instr.f_name = name
		//new_func := function{name: name,instructs: new_bycode}
		//FUNCTS[name] = new_func
		return counter,instr
	} else {
		KILL = true
		fmt.Println("switch dosen’t have a body:",counter)
		return counter,instr
	}
}


func make_bytecode(code string) []instruction {
	var bytecode []instruction 
	var counter uint64 = 0
	LOOPS := make([]uint64,0)
	var code_len uint64 = uint64(len(code))
	for counter < code_len {
		key := code[counter]
		switch key {
			default:
			counter += 1
			case '+':
			var temp instruction
			counter, temp = comp_add_sub_move(counter,code,key)
			bytecode = append(bytecode, temp)
			case '-':
			var temp instruction
			counter, temp = comp_add_sub_move(counter,code,key)
			bytecode = append(bytecode, temp)
			case '<':
			var temp instruction
			counter, temp = comp_add_sub_move(counter,code,key)
			bytecode = append(bytecode, temp)
			case '>':
			var temp instruction
			counter, temp = comp_add_sub_move(counter,code,key)
			bytecode = append(bytecode, temp)
			case '[':
			LOOPS = append(LOOPS, uint64(len(bytecode)))
			temp := instruction{instr: 4 , other: counter}
			bytecode = append(bytecode, temp)
			counter += 1
			case ']':
			var temp uint64
			temp, LOOPS = LOOPS[len(LOOPS)-1], LOOPS[:len(LOOPS)-1]
			bytecode[temp].value = uint64(len(bytecode))
			temp_2 := instruction{instr: 5,value: temp}
			bytecode = append(bytecode, temp_2)
			counter += 1
			case '.':
			var temp instruction
			counter, temp = comp_print(counter,code)
			bytecode = append(bytecode, temp)
			case ',':
			temp := instruction{instr: 7}
			bytecode = append(bytecode, temp)
			counter += 1
			case '!':
			var temp instruction
			counter,temp = comp_func(counter,code)
			bytecode = append(bytecode, temp)
			case '?':
			var temp instruction
			counter,temp = comp_load_func(counter,code)
			bytecode = append(bytecode, temp)
			
		}
		if KILL { return bytecode}
	}
	if len(LOOPS) > 0 {
		KILL = true
		var temp uint64
		temp, LOOPS = LOOPS[len(LOOPS)-1], LOOPS[:len(LOOPS)-1]
		fmt.Println("Loop not closed:",bytecode[temp].other)
	}
	return bytecode
}

var POINTER_POS uint16 = 0
var REGISTER []byte

func interpret_bytecode(bytecode []instruction){
	var counter uint64 = 0
	//POINTER_POS = 0
	// REGISTER = make([]byte,0xffff)
	// LOOPS := make([]uint64,0)
	var code_len uint64 = uint64(len(bytecode))
	for counter < code_len {
		instr := bytecode[counter]
		//print_inst(instr)
		switch instr.instr {
			default:
			counter += 1
			case 0:
			REGISTER[POINTER_POS] += byte(instr.value)
			counter += 1
			case 1:
			REGISTER[POINTER_POS] -= byte(instr.value)
			counter += 1
			case 2:
			POINTER_POS -= uint16(instr.value)
			counter += 1
			case 3:
			POINTER_POS += uint16(instr.value)
			counter += 1
			case 4:
			if REGISTER[POINTER_POS] == 0 {
				counter = instr.value
			}
			counter += 1
			case 5:
			counter = instr.value
			case 6:
			if instr.value == 0 {
				fmt.Print(string(REGISTER[POINTER_POS]))
			} else if instr.value == 1 {
				fmt.Print(REGISTER[POINTER_POS])
			} else if instr.value == 2 {
				fmt.Printf("0x%02x",REGISTER[POINTER_POS])
			} else if instr.value == 3 {
				fmt.Printf("0b%08b",REGISTER[POINTER_POS])
			}
			counter += 1
			case 7:
			fmt.Print("\n> ")
			REGISTER[POINTER_POS] = pollKey()
			counter += 1
			case 9:
			n_func := FUNCTS[instr.f_name]
			interpret_bytecode(n_func.instructs)
			counter += 1
		}
	}
}

func print_bytecode(bytecode []instruction, is_in_func bool) {
	//fmt.Println()
	for _,ele := range bytecode {
		//var instr string
		if is_in_func{
			fmt.Print("  ")
		}
		print_inst(ele)
	}
}
func print_inst(ele instruction) {
	switch ele.instr{
		case 0:
		fmt.Print("add ",ele.value,"\n")
		case 1:
		fmt.Print("sub ",ele.value,"\n")
		case 2:
		fmt.Print("move_left ",ele.value,"\n")
		case 3:
		fmt.Print("move_right ",ele.value,"\n")
		case 4:
		fmt.Print("start_loop ",ele.value,"\n")
		case 5:
		fmt.Print("end_loop ",ele.value,"\n")
		case 6:
		fmt.Print("print ",ele.value,"\n")
		case 7:
		fmt.Print("read ",ele.value,"\n")
		case 8:
		fmt.Print("create_func ",ele.f_name,"\n")
		case 9:
		fmt.Print("create_func ",ele.f_name,"\n")
	}
}

var print_bycode = false

func main() {
	//fmt.Println(len(os.Args), os.Args[1])

	if len(os.Args) == 1 {
		fmt.Println("No input given")
		return
	}
	termInit()
	defer termRestore()
	for i,ele := range os.Args{
		if i == 0 {continue} else if i == len(os.Args)-1 {continue}
		switch ele {
			case "-d":
			print_bycode = true
		}
		
	}
	
	content, err := os.ReadFile(os.Args[len(os.Args)-1])
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	//fmt.Println(string(content))
	//interpret_bf(string(content))
	code := string(content)
	//code += "  "
	FUNCTS = make(map[string]function)
	var bytecode []instruction 
	bytecode = make_bytecode(string(code))
	if KILL {
		return
	}
	if print_bycode {
		print_bytecode(bytecode,false)
		if len(FUNCTS) > 0 {
			for i,ele := range FUNCTS {
				fmt.Println()
				fmt.Printf(":%s\n",i)
				print_bytecode(ele.instructs,true)
			}
		}
	}
	REGISTER = make([]byte,0xffff)
	start_func,ok := FUNCTS["main"]
	if ok {
		interpret_bytecode(start_func.instructs)
	} else {
		fmt.Println("No main function found")
	}
	fmt.Println()
}