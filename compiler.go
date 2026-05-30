package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"runtime"
)

// ── C backend ─────────────────────────────────────────────────────────────────

func compile_to_c(bytecode []instruction, out_name string) {
	var sb strings.Builder
	
	sb.WriteString("#include <stdio.h>\n")
	sb.WriteString("#include <stdint.h>\n")

	if runtime.GOOS == "windows" {
		sb.WriteString("#include <windows.h>\n\n")
		
		sb.WriteString("DWORD orig_mode;\n")
		sb.WriteString("HANDLE hStdin;\n\n")
		
		sb.WriteString("void term_init() {\n")
		sb.WriteString("  hStdin = GetStdHandle(STD_INPUT_HANDLE);\n")
		sb.WriteString("  GetConsoleMode(hStdin, &orig_mode);\n\n")
		sb.WriteString("  DWORD raw_mode = orig_mode;\n")
		sb.WriteString("  raw_mode &= ~(ENABLE_LINE_INPUT | ENABLE_ECHO_INPUT);\n")
		sb.WriteString("  raw_mode &= ~ENABLE_QUICK_EDIT_MODE;\n\n")
		sb.WriteString("  SetConsoleMode(hStdin, raw_mode);\n")
		sb.WriteString("}\n\n")
		
		sb.WriteString("void term_restore() {\n")
		sb.WriteString("  SetConsoleMode(hStdin, orig_mode);\n")
		sb.WriteString("}\n\n")
	} else {
		
		sb.WriteString("#include <termios.h>\n")
		sb.WriteString("#include <unistd.h>\n\n")
	
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
	}
	
	for name := range TAPES {
		sb.WriteString(fmt.Sprintf("uint8_t tape_%s[0xffff] = {0};\n", name))
		sb.WriteString(fmt.Sprintf("uint16_t ptr_%s = 0;\n", name))
	}
	
	sb.WriteString("\n")
	
	sb.WriteString("uint8_t *reg = tape_main;\n")
	sb.WriteString("uint16_t *ptr = &ptr_main;\n")

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
