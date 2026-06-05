

CODE : str ='''>++++++++[<+++++++++>-]<.>++++[<+++++++>-]<+.+++++++..+++.>>++++++[<+++++++>-]<+
            +.------------.>++++++[<+++++++++>-]<+.<.+++.------.--------.>>>++++[<++++++++>-
            ]<+.
            ++++++++++++++++++++++++++++++++++++++++++>>+++++<++++++++++-----[-]++++>[-]++>[-]>[-]++++++>[-]+++++++++<<<<.1>.1>.1>.1>.1>[-]++++>[-]++>[-]>[-]++++++>[-]+++++++++<<<<.1>.1>.1>.1>.1>+++++.1>++++++++++.1''' #'+>++>+++>++++.<.<.<.[.[-.]>]'
POINTER_POS : int = 0
REGISTER : list[int] = [0]

def move_left():
    global POINTER_POS
    POINTER_POS -= 1 if POINTER_POS != 0 else 0

def move_right():
    global POINTER_POS,REGISTER
    if POINTER_POS == len(REGISTER)-1:
        REGISTER.append(0)
    POINTER_POS += 1

def increce_register():
    global REGISTER
    REGISTER[POINTER_POS] += 1
    REGISTER[POINTER_POS] = REGISTER[POINTER_POS] % 256

def decrece_register():
    global REGISTER
    REGISTER[POINTER_POS] -= 1
    REGISTER[POINTER_POS] = REGISTER[POINTER_POS] % 256

def register_out():
    print(REGISTER[POINTER_POS],chr(REGISTER[POINTER_POS]))




def interp_bf(code : str):
    counter = 0
    while counter < len(code):
        if code[counter] =='+':
            increce_register()
        elif code[counter] == '-':
            decrece_register()
        elif code[counter] == '<':
            move_left()
        elif code[counter] == '>':
            move_right()
        elif code[counter] == '.':
            register_out()
        elif code[counter] == ']':
            loop_count = 1
            if REGISTER[POINTER_POS] == 0:
                pass
            else:
                #print('counter:', counter, 'functoin:', code[counter])
                while not(code[counter] == '[' and loop_count == 0):
                    #print(
                    #   'loop status:',not(code[counter] == '[' and loop_count == 0),
                    #)
                    counter -= 1
                    #print(
                    #     'loop status after:', not (code[counter] == '[' and loop_count == 0),
                    #     'stets(','counter:', counter, 'functoin:', code[counter],
                    #     'loop count:', loop_count,')'
                    #)
                    if code[counter] == ']':
                        loop_count +=1
                    elif code[counter] == '[' and loop_count > 0:
                        loop_count -= 1
                else:
                    pass
                    #print('start of loop','counter:', counter, 'functoin:', code[counter])


        counter += 1
        #print('counter:',counter,'functoin:',code[counter])

if __name__ == '__main__':
    with open('test.bfpp','r') as f:
        CODE = f.read()
    interp_bf(CODE)