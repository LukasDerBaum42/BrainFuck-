import brainfuck_interpreter

CODE_IN = '+42>2+5<+10-5!(lol)[-]+4>[-]+2>[-]>[-]+6>[-]+9<4.>.>.>.>.!?(lol)>?(lol)> (lul=5)+(lul).>+(lul=10).'
CODE_OUT = ''

NUMBERS = ['0','1','2','3','4','5','6','7','8','9']
OPS = ['+','-','.','<','>','[',']','!','?','(',')','=']
NOR_OPS = ['.','[',']']
NUM_OPS = ['+','-','>','<']
FUNCS = {}
VAR = {}

def get_num(code,pos) :
    num = ''
    while code[pos] in NUMBERS:
        num += code[pos]
        pos +=1 if pos+1 < len(code) else 0
    pos -= 1
    num = int(num) if num != '' else 1
    return num , pos

#print(get_num(CODE_IN,1))



def do_num_op(code,pos):
    op = ''
    num = 1
    if code[pos] in NUM_OPS:
        op = code[pos]
    pos += 1
    if code[pos] == '(':
        num, pos =rw_var(code,pos)
    else:
        num , pos = get_num(code,pos)
    comp = op * num
    return comp , pos

def get_bracked(code,pos):
    content = ''
    while code[pos] != ')':
        content += code[pos]
        pos += 1
    return content,pos


def create_func(code, pos):
    global FUNCS
    key_word = ''
    pos += 1
    if code[pos] == '(':
        pos += 1
        key_word, pos = get_bracked(code,pos)
        pos += 1
        func = ''
        while code[pos] != '!':
            func += code[pos]
            pos += 1
        FUNCS[key_word] = func
    else:
        OSError('NO open bracert found at:',pos)
    return pos

def rw_var(code,pos):
    global VAR
    pos += 1
    key_word = ''
    while not(code[pos] == ')' or code[pos] == '='):
        key_word += code[pos]
        pos += 1
    print(key_word)
    if code[pos] == ')':
        print('r',VAR[key_word])
        return VAR[key_word] , pos
    elif code[pos] == '=':
        pos += 1
        num , pos = get_num(code,pos)
        print('w',num)
        VAR[key_word] = num
        return num, pos
    return 0,pos


def call_func(code,pos):
    key_word = ''
    pos += 1
    if code[pos] == '(':
        pos += 1
        key_word, pos = get_bracked(code, pos)
        func = ''
        func = FUNCS[key_word]
        counter = 0
        comp = ''
        while counter < len(func):
            if func[counter] in OPS:
                if func[counter] in NUM_OPS:
                    comp_temp, counter = do_num_op(func, counter)
                    comp += comp_temp
                elif func[counter] in NOR_OPS:
                    comp += func[counter]
                elif func[counter] == '?':
                    comp_temp, counter = call_func(func, counter)
                    comp += comp_temp
                elif func[counter] == '(':
                    _, counter = rw_var(func, counter)
            counter += 1
        return comp, pos
    else:
        OSError('NO open bracert found at:',pos)
        return '', pos

def compile_to_bf(code):
    code += ' '
    global CODE_OUT
    counter = 0
    while counter < len(code):
        if code[counter] in OPS:
            if code[counter] in NUM_OPS:
                comp_temp , counter = do_num_op(code,counter)
                CODE_OUT += comp_temp
            elif code[counter] in NOR_OPS:
                CODE_OUT += code[counter]
            elif code[counter] == '?':
                comp_temp,counter = call_func(code,counter)
                CODE_OUT += comp_temp
            elif code[counter] == '(':
                _ , counter = rw_var(code,counter)
            elif code[counter] == '!':
                counter = create_func(code,counter)

        counter += 1



#compile_to_bf(CODE_IN)
#print(CODE_OUT)
if __name__ == '__main__':
    with open('brainpy_code.txt') as f:
        CODE_IN = f.read()
    compile_to_bf(CODE_IN)
    print(CODE_OUT)
    brainfuck_interpreter.interp_bf(CODE_OUT)
    with open('brainfuck_code.txt','w') as f:
        f.write(CODE_OUT)

