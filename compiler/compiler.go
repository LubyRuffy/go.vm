//
// This is the "compiler" for our simple virtual machine.
//
// It reads the string of tokens from the lexer, and outputs the bytecode
// which is equivalent.
//
// The approach to labels is the same as in the inspiring-project:  Every time
// we come across a label we output a pair of temporary bytes in our bytecode.
// Later, once we've read the whole program and assume we've found all existing
// labels,  we go back up and fix the generated addresses.
//
// This mechanism is the reason for the `fixups` and `labels` maps in the
// Compiler object - the former keeps track of offsets in our generated
// bytecodes that need to be patched with the address/offset of a given
// label, and the latter lets us record the offset at which labels were seen.
//
//

package compiler

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/skx/go.vm/lexer"
	"github.com/skx/go.vm/opcode"
	"github.com/skx/go.vm/token"
)

// Compiler contains our compiler-state
type Compiler struct {
	l         *lexer.Lexer   // our lexer
	curToken  token.Token    // current token
	peekToken token.Token    // next token
	bytecode  []byte         // generated bytecode
	labels    map[string]int // holder for labels
	fixups    map[int]string // holder for fixups
}

// New is our constructor
func New(l *lexer.Lexer) *Compiler {
	p := &Compiler{l: l}
	p.labels = make(map[string]int)
	p.fixups = make(map[int]string)

	// prime the pump.
	p.nextToken()
	p.nextToken()
	return p
}

// nextToken gets the next token from our lexer-stream
func (p *Compiler) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// isRegister returns true if the given string has a register ID
func (p *Compiler) isRegister(input string) bool {
	if strings.HasPrefix(input, "#") {
		return true
	}
	return false
}

// getRegister converts a register string "#2" to an integer 2.
func (p *Compiler) getRegister(input string) byte {

	num := strings.TrimPrefix(input, "#")

	i, err := strconv.Atoi(num)
	if err != nil {
		panic(err)
	}

	if (i >= 0) && (i <= 15) {
		return byte(i)
	}

	fmt.Printf("Register out of bounds: #%s\n", input)
	os.Exit(1)
	return 0
}

// Dump processe the stream of tokens from the lexer and shows the structure
// of the program.
func (p *Compiler) Dump() {

	// Until we get the end of our stream we'll show each token.
	for p.curToken.Type != token.EOF {
		fmt.Printf("%v\n", p.curToken)
		p.nextToken()
	}
}

// Compile processe the stream of tokens from the lexer and builds
// up the bytecode program.
func (p *Compiler) Compile() {

	// Until we get the end of our stream we'll process each token
	// in turn, generating bytecode as we go.
	for p.curToken.Type != token.EOF {

		// Now handle the various tokens
		switch p.curToken.Type {

		case token.LABEL:
			// Remove the ":" prefix from the label
			label := strings.TrimPrefix(p.curToken.Literal, ":")
			// The label points to the current point in our bytecode
			p.labels[label] = len(p.bytecode)

		case token.EXIT:
			p.exitOp()

		case token.INC:
			p.incOp()

		case token.DEC:
			p.decOp()

		case token.RANDOM:
			p.randOp()

		case token.RET:
			p.retOp()

		case token.CALL:
			p.callOp()

		case token.IS_INTEGER:
			p.isIntOp()

		case token.IS_STRING:
			p.isStrOp()

		case token.STRING2INT:
			p.str2IntOp()

		case token.INT2STRING:
			p.int2StrOp()

		case token.SYSTEM:
			p.systemOp()

		case token.CMP:
			p.cmpOp()

		case token.CONCAT:
			p.concatOp()

		case token.DB:
			p.dataOp()

		case token.DATA:
			p.dataOp()

		case token.GOTO:
			p.jumpOp(opcode.JUMP_TO)

		case token.TRAP:
			p.trapOp()

		case token.JMP:
			p.jumpOp(opcode.JUMP_TO)

		case token.JMPZ:
			p.jumpOp(opcode.JUMP_Z)

		case token.JMPNZ:
			p.jumpOp(opcode.JUMP_NZ)

		case token.MEMCPY:
			p.memcpyOp()

		case token.NOP:
			p.nopOp()

		case token.PEEK:
			p.peekOp()

		case token.POKE:
			p.pokeOp()

		case token.PUSH:
			p.pushOp()

		case token.POP:
			p.popOp()

		case token.STORE:
			p.storeOp()

		case token.PRINT_INT:
			p.printInt()

		case token.PRINT_STR:
			p.printString()

		case token.ADD:
			p.mathOperation(opcode.ADD_OP)

		case token.XOR:
			p.mathOperation(opcode.XOR_OP)

		case token.SUB:
			p.mathOperation(opcode.SUB_OP)

		case token.MUL:
			p.mathOperation(opcode.MUL_OP)

		case token.DIV:
			p.mathOperation(opcode.DIV_OP)

		case token.AND:
			p.mathOperation(opcode.AND_OP)

		case token.OR:
			p.mathOperation(opcode.OR_OP)

		default:
			fmt.Println("Unhandled token: ", p.curToken)

		}
		p.nextToken()
	}

	// Now fixup any label-names we've got to patch into place.
	for addr, name := range p.fixups {
		value := p.labels[name]
		if value == 0 {
			fmt.Printf("Possible use of undefined label '%s'\n", name)
		}

		p1 := value % 256
		p2 := (value - p1) / 256

		p.bytecode[addr] = byte(p1)
		p.bytecode[addr+1] = byte(p2)
	}
}

// nopOp does nothing
func (p *Compiler) nopOp() {
	p.bytecode = append(p.bytecode, byte(opcode.NOP_OP))
}

// peekOp reads the contents of a memory address, and stores in a register
func (p *Compiler) peekOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	res := p.getRegister(p.curToken.Literal)

	// now we have a comma
	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	// and a literal
	if p.curToken.Type != token.IDENT {
		return
	}
	addr := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.PEEK))
	p.bytecode = append(p.bytecode, byte(res))
	p.bytecode = append(p.bytecode, byte(addr))

}

// pokeOp writes to memory
func (p *Compiler) pokeOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	val := p.getRegister(p.curToken.Literal)

	// now we have a comma
	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	// and a literal
	if p.curToken.Type != token.IDENT {
		return
	}
	addr := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.POKE))
	p.bytecode = append(p.bytecode, byte(val))
	p.bytecode = append(p.bytecode, byte(addr))
}

// pushOp stores a stack-push
func (p *Compiler) pushOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.STACK_PUSH))
	p.bytecode = append(p.bytecode, byte(reg))
}

// popOp stores a stack-push
func (p *Compiler) popOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.STACK_POP))
	p.bytecode = append(p.bytecode, byte(reg))
}

// exitOp terminates our interpeter
func (p *Compiler) exitOp() {
	p.bytecode = append(p.bytecode, byte(opcode.EXIT))
}

// incOp increments the contents of the given register
func (p *Compiler) incOp() {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.INC_OP))
	p.bytecode = append(p.bytecode, byte(reg))
}

// decOp decrements the contents of the given register
func (p *Compiler) decOp() {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.DEC_OP))
	p.bytecode = append(p.bytecode, byte(reg))
}

// randOp returns a random value
func (p *Compiler) randOp() {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.INT_RANDOM))
	p.bytecode = append(p.bytecode, byte(reg))
}

// retOp returns from a call
func (p *Compiler) retOp() {
	p.bytecode = append(p.bytecode, byte(opcode.STACK_RET))
}

// isStrOp tests if a register contains a string
func (p *Compiler) isStrOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.IS_STRING))
	p.bytecode = append(p.bytecode, byte(reg))
}

// str2IntOp converts the given string-register to an int.
func (p *Compiler) str2IntOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.STRING_TOINT))
	p.bytecode = append(p.bytecode, byte(reg))
}

// int2StrOp converts the given int-register to a string.
func (p *Compiler) int2StrOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.INT_TOSTRING))
	p.bytecode = append(p.bytecode, byte(reg))
}

// systemOp runs the (string) command in the given register
func (p *Compiler) systemOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.STRING_SYSTEM))
	p.bytecode = append(p.bytecode, byte(reg))
}

// isIntOp tests if a register contains an integer
func (p *Compiler) isIntOp() {
	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(opcode.IS_INTEGER))
	p.bytecode = append(p.bytecode, byte(reg))
}

// callOp generates a call instruction
func (p *Compiler) callOp() {

	// add the call instruction
	p.bytecode = append(p.bytecode, byte(opcode.STACK_CALL))
	// advance to the target
	p.nextToken()

	// The call might be to an absolute target, or a label.
	switch p.curToken.Type {

	case token.INT:
		addr, _ := strconv.ParseInt(p.curToken.Literal, 0, 64)

		len1 := addr % 256
		len2 := (addr - len1) / 256

		p.bytecode = append(p.bytecode, byte(len1))
		p.bytecode = append(p.bytecode, byte(len2))

	case token.IDENT:

		// Record that we have to fixup this thing
		p.fixups[len(p.bytecode)] = p.curToken.Literal

		// output two temporary numbers
		p.bytecode = append(p.bytecode, byte(0))
		p.bytecode = append(p.bytecode, byte(0))
	}

}

// trapOp inserts an interrupt call / trap
func (p *Compiler) trapOp() {

	// advance to the target
	p.nextToken()

	// The jump might be an absolute target, or a label.
	switch p.curToken.Type {

	case token.INT:
		addr, _ := strconv.ParseInt(p.curToken.Literal, 0, 64)
		len1 := addr % 256
		len2 := (addr - len1) / 256

		p.bytecode = append(p.bytecode, byte(opcode.TRAP_OP))
		p.bytecode = append(p.bytecode, byte(len1))
		p.bytecode = append(p.bytecode, byte(len2))
	default:
		fmt.Printf("Fail!")
	}
}

// jumpOp inserts a direct jump
func (p *Compiler) jumpOp(operator int) {

	// add the jump
	p.bytecode = append(p.bytecode, byte(operator))

	// advance to the target
	p.nextToken()

	// The jump might be an absolute target, or a label.
	switch p.curToken.Type {

	case token.INT:
		addr, _ := strconv.ParseInt(p.curToken.Literal, 0, 64)
		len1 := addr % 256
		len2 := (addr - len1) / 256

		p.bytecode = append(p.bytecode, byte(len1))
		p.bytecode = append(p.bytecode, byte(len2))

	case token.IDENT:

		// Record that we have to fixup this thing
		p.fixups[len(p.bytecode)] = p.curToken.Literal

		// output two temporary numbers
		p.bytecode = append(p.bytecode, byte(0))
		p.bytecode = append(p.bytecode, byte(0))
	}

}

// memcpyOp inserts a memcopy operation.
func (p *Compiler) memcpyOp() {
	p.nextToken()

	one := p.getRegister(p.curToken.Literal)

	if !p.expectPeek(token.COMMA) {
		return
	}

	p.nextToken()
	two := p.getRegister(p.curToken.Literal)

	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	three := p.getRegister(p.curToken.Literal)

	// output the bytecode
	p.bytecode = append(p.bytecode, byte(opcode.MEMCPY))
	p.bytecode = append(p.bytecode, byte(one))
	p.bytecode = append(p.bytecode, byte(two))
	p.bytecode = append(p.bytecode, byte(three))
}

// mathOperation handles add/sub/mul/div/etc
func (p *Compiler) mathOperation(operation int) {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// dest
	dst := p.getRegister(p.curToken.Literal)

	// now we have a comma
	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	// and a literal
	if p.curToken.Type != token.IDENT {
		return
	}
	src1 := p.getRegister(p.curToken.Literal)

	// and a comma
	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	// and a final literal
	if p.curToken.Type != token.IDENT {
		return
	}
	src2 := p.getRegister(p.curToken.Literal)

	p.bytecode = append(p.bytecode, byte(operation))
	p.bytecode = append(p.bytecode, byte(dst))
	p.bytecode = append(p.bytecode, byte(src1))
	p.bytecode = append(p.bytecode, byte(src2))

}

// storeOp handles loading a register with a string, integer, or register,
// or label-address.
func (p *Compiler) storeOp() {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	// Now we know where we're storing the thing we need to determine
	// what is being stored: string, integer, register value, or a
	// label address.
	switch p.curToken.Type {

	case token.STRING:
		// STRING_STORE $REG $LEN1 $LEN2 $STRING
		p.bytecode = append(p.bytecode, byte(opcode.STRING_STORE))
		p.bytecode = append(p.bytecode, reg)

		len := len(p.curToken.Literal)

		len1 := len % 256
		len2 := (len - len1) / 256
		p.bytecode = append(p.bytecode, byte(len1))
		p.bytecode = append(p.bytecode, byte(len2))

		// output the length
		for i := 0; i < len; i++ {
			p.bytecode = append(p.bytecode, byte(p.curToken.Literal[i]))
		}
	case token.INT:
		// INT_STORE $REG $NUM1 NUM2
		p.bytecode = append(p.bytecode, byte(opcode.INT_STORE))
		p.bytecode = append(p.bytecode, reg)

		// Convert to low/high
		i, _ := strconv.ParseInt(p.curToken.Literal, 0, 64)
		len1 := i % 256
		len2 := (i - len1) / 256
		p.bytecode = append(p.bytecode, byte(len1))
		p.bytecode = append(p.bytecode, byte(len2))
	case token.IDENT:
		if p.isRegister(p.curToken.Literal) {
			// REG_STORE REG_DST REG_SRC
			p.bytecode = append(p.bytecode, byte(opcode.REG_STORE))
			p.bytecode = append(p.bytecode, reg)
			p.bytecode = append(p.bytecode, p.getRegister(p.curToken.Literal))
		} else {
			// Here we're storing the address of a label.

			// INT_STORE $REG $NUM1 $NUM2
			p.bytecode = append(p.bytecode, byte(opcode.INT_STORE))
			p.bytecode = append(p.bytecode, reg)

			// record that we need a fixup here
			p.fixups[len(p.bytecode)] = p.curToken.Literal

			// output two temporary numbers
			p.bytecode = append(p.bytecode, byte(0))
			p.bytecode = append(p.bytecode, byte(0))
		}
	default:
		fmt.Printf("ERROR: Invalid thing to store: %v\n", p.curToken)
		os.Exit(1)
	}
}

// cmpOp handles comparing a register with a string, integer, or register,
// or label-address.
func (p *Compiler) cmpOp() {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	// Save the register we're storing to.
	reg := p.getRegister(p.curToken.Literal)

	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	// Now we know what source register we're comparing we need to see
	// if that comparison is with a string, integer, register value, or a
	// label address.
	switch p.curToken.Type {

	case token.STRING:
		// CMP_STRING $REG $LEN1 $LEN2 $STRING
		p.bytecode = append(p.bytecode, byte(opcode.CMP_STRING))
		p.bytecode = append(p.bytecode, reg)

		len := len(p.curToken.Literal)

		len1 := len % 256
		len2 := (len - len1) / 256
		p.bytecode = append(p.bytecode, byte(len1))
		p.bytecode = append(p.bytecode, byte(len2))

		// append the string
		for i := 0; i < len; i++ {
			p.bytecode = append(p.bytecode, byte(p.curToken.Literal[i]))
		}
	case token.INT:
		// CMP_IMMEDIATE $REG $NUM1 NUM2
		p.bytecode = append(p.bytecode, byte(opcode.CMP_IMMEDIATE))
		p.bytecode = append(p.bytecode, reg)

		// Convert to low/high
		i, _ := strconv.ParseInt(p.curToken.Literal, 0, 64)

		len1 := i % 256
		len2 := (i - len1) / 256
		p.bytecode = append(p.bytecode, byte(len1))
		p.bytecode = append(p.bytecode, byte(len2))
	case token.IDENT:
		if p.isRegister(p.curToken.Literal) {
			// CMP_REG REG_DST REG_SRC
			p.bytecode = append(p.bytecode, byte(opcode.CMP_REG))
			p.bytecode = append(p.bytecode, reg)
			p.bytecode = append(p.bytecode, p.getRegister(p.curToken.Literal))
		} else {
			// Here we're storing the address of a label.

			// INT_STORE $REG $NUM1 $NUM2
			p.bytecode = append(p.bytecode, byte(opcode.CMP_IMMEDIATE))
			p.bytecode = append(p.bytecode, reg)

			// record that we need a fixup here
			p.fixups[len(p.bytecode)] = p.curToken.Literal

			// output two temporary numbers
			p.bytecode = append(p.bytecode, byte(0))
			p.bytecode = append(p.bytecode, byte(0))
		}
	default:
		fmt.Printf("ERROR: Invalid thing to store: %v\n", p.curToken)
		os.Exit(1)
	}
}

// concatOp concatenates two string values.
func (p *Compiler) concatOp() {
	p.nextToken()

	dst := p.getRegister(p.curToken.Literal)

	if !p.expectPeek(token.COMMA) {
		return
	}

	p.nextToken()
	a := p.getRegister(p.curToken.Literal)

	if !p.expectPeek(token.COMMA) {
		return
	}
	p.nextToken()

	b := p.getRegister(p.curToken.Literal)

	// output the bytecode
	p.bytecode = append(p.bytecode, byte(opcode.STRING_CONCAT))
	p.bytecode = append(p.bytecode, byte(dst))
	p.bytecode = append(p.bytecode, byte(a))
	p.bytecode = append(p.bytecode, byte(b))
}

// dataOp embeds literal/binary data into the output
func (p *Compiler) dataOp() {
	p.nextToken()

	// We might have a string, or a series of ints
	//
	// If it is a string handle that first
	if p.curToken.Type == token.STRING {
		len := len(p.curToken.Literal)
		for i := 0; i < len; i++ {
			p.bytecode = append(p.bytecode, byte(p.curToken.Literal[i]))
		}
		return
	}

	//
	// Otherwise we expect a single int
	//
	db := p.curToken.Literal
	i, _ := strconv.ParseInt(db, 0, 64)
	p.bytecode = append(p.bytecode, byte(i))

	//
	// Loop looking for more data - we don't know how much
	// there might be, but we'll know it is comma-separated.
	//
	for p.peekTokenIs(token.COMMA) {
		// skip the comma
		p.nextToken()

		// read the next int
		if p.expectPeek(token.INT) {
			db := p.curToken.Literal
			i, _ := strconv.ParseInt(db, 0, 64)
			p.bytecode = append(p.bytecode, byte(i))
		}
	}
}

// Handle printing the contents of a register as an integer.
func (p *Compiler) printInt() {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	p.bytecode = append(p.bytecode, byte(opcode.INT_PRINT))
	p.bytecode = append(p.bytecode, p.getRegister(p.curToken.Literal))
}

// Handle printing the contents of a register as a string.
func (p *Compiler) printString() {

	// We're looking for an identifier next.
	if !p.expectPeek(token.IDENT) {
		return
	}

	p.bytecode = append(p.bytecode, byte(opcode.STRING_PRINT))
	p.bytecode = append(p.bytecode, p.getRegister(p.curToken.Literal))
}

// determinate current token is t or not.
func (p *Compiler) curTokenIs(t token.Type) bool {
	return p.curToken.Type == t
}

// determinate next token is t or not
func (p *Compiler) peekTokenIs(t token.Type) bool {
	return p.peekToken.Type == t
}

// expect next token is t
// succeed: return true and forward token
// failed: return false and store error
func (p *Compiler) expectPeek(t token.Type) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Compiler) peekError(t token.Type) {
	fmt.Printf("expected next token to be %s, got %s instead", t, p.curToken.Type)
	os.Exit(1)
}

// Write outputs our generated bytecode to the named file.
func (p *Compiler) Write(output string) {
	fmt.Printf("Our bytecode is %d bytes long\n", len(p.bytecode))
	err := ioutil.WriteFile(output, p.bytecode, 0644)
	if err != nil {
		fmt.Printf("Error writing output file: %s\n", err.Error())
		os.Exit(1)
	}
}

// Output returns the bytecodes of the compiled program.
func (p *Compiler) Output() []byte {
	return (p.bytecode)
}
