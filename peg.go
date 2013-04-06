/*
Package peg is a PEG-based parser.
*/
package peg

import (
    "fmt"
    "strings"
    "unicode"
)

type Parser struct {
    // for debugging
    info string
    // if false, allows fast rejection of empty strings by invocation wrapper
    allowEmpty bool
    // if true all subsidiary parsers don't skip whitespace
    adjacent bool
    // actual parse function, returns whether matched + amount of input consumed
    parse func(state *state, input []rune) (bool, int)
    // if a compound parser, these are subsidiary parsers
    subParsers []*Parser
    // what to call with the string matched by this parser
    handler func(result string)
}

type state struct {
    // if == 0 we skip leading whitespace before invoking parser function
    skipWhite int
    // recursion depth
    depth int
}

// Return a Parser that matches any character in a string.
//
func AnyOf(str string) *Parser {
    return newParser("Anyof(" + str + ")", false, nil, func(state *state, input []rune) (bool, int) {
        // TODO: optimize
        for _, char := range str {
            if input[0] == char {
                return true, 1
            }
        }
        return false, 0
    })
}

// Return a Parser that matches a literal string in the input.
//
func Literal(str string) *Parser {
    runes := []rune(str)
    strLen := len(runes)
    return newParser("Literal(" + str + ")", len(str) == 0, nil, func(state *state, input []rune) (bool, int) {
        inputLen := len(input)
        if strLen > inputLen {
            return false, 0
        }
        for i, char := range runes {
            if char != input[i] {
                return false, 0
            }
        }
        return true, strLen
    })
}

// Return a Parser that tries a list of parsers in succession
// and stops after the first that matches.
//
func OneOf(parsers ...*Parser) *Parser {
    return newParser("OneOf", false, parsers, func(state *state, input[]rune) (bool, int) {
        for _, parser := range parsers {
            match, used := parser.invoke(state, input)
            if match {
                return match, used
            }
        }
        return false, 0
    })
}

// Like ZeroOrMoreOf but must match at least one, once.
//
func OneOrMoreOf(parsers ...*Parser) *Parser {
    return newParser("OneOrMoreOf", false, parsers, func(state *state, input[]rune) (bool, int) {
        return oneOrMoreOf(state, parsers, input)
    })
}

func oneOrMoreOf(state *state, parsers []*Parser, input[] rune) (bool, int) {
    for _, parser := range parsers {
        match, used :=  parser.invoke(state, input)
        if match {
            _, used2 := oneOrMoreOf(state, parsers, input[used:])
            return true, used + used2
        }
    }
    return false, 0
}

// Return a parser that optionally matches what another parser matches.
//

func Optional(parser *Parser) *Parser {
    return newParser("OneOrMoreOf", true, []*Parser{parser}, func(state *state, input[]rune) (bool, int) {
        match, used := parser.invoke(state, input)
        if match {
            return match, used
        }
        return true, 0
    })
}

// Return a parser that matches if each of the supplied parsers
// matches when tried in succession.
//
func Sequence(parsers ...*Parser) *Parser {
    return newParser("Sequence", false, parsers, func(state *state, input []rune) (bool, int) {
        total := 0
        for _, parser := range parsers {
            match, used := parser.invoke(state, input)
            if ! match {
                return false, 0
            }
            input = input[used:]
            total += used
        }
        return true, total
    })
}

// Return a Parser that keeps matching as long as any parser
// in the supplied list of parsers matches.
//
func ZeroOrMoreOf(parsers ...*Parser) *Parser {
    return newParser("ZeroOrMoreOf", true, parsers, func(state *state, input[]rune) (bool, int) {
        return zeroOrMoreOf(state, parsers, input)
    })
}

func zeroOrMoreOf(state *state, parsers []*Parser, input[] rune) (bool, int) {
    for _, parser := range parsers {
        match, used :=  parser.invoke(state, input)
        if match {
            _, used2 := zeroOrMoreOf(state, parsers, input[used:])
            return true, used + used2
        }
    }
    return true, 0
}

// Returns a modified parser whose handler will be passed
// Creates a Parser node around a parsing function.
//
func newParser(info string, allowEmpty bool, subParsers []*Parser, 
               parse func(state *state, input []rune) (bool, int)) *Parser {
    return &Parser{info, allowEmpty, false, parse, subParsers, nil}
}

// Run one pass of a parser.  Skips whitespace if directed, and invokes
// the handler with the string matched.
//
func (parser *Parser) invoke(state *state, input []rune) (bool, int) {
    fmt.Printf("%s-> %s on '%s'\n", strings.Repeat(" ", state.depth * 2), 
                parser.info, string(input))
    // TODO: optimize to skip space once per each change of depth; right now
    // for e.g. OneOf we do it for each subsidiary match attempt
    space := parser.skipWhite(state, input)
    if space > 0 {
        input = input[space:]
    }
    if len(input) == 0 && !parser.allowEmpty {
        return false, 0
    }
    state.depth += 1
    if parser.adjacent {
        state.skipWhite += 1
    }
    match, used := parser.parse(state, input)
    state.depth -= 1
    if parser.adjacent {
        state.skipWhite -= 1
    }
    fmt.Printf("%s%v, %d <- %s on '%s'\n", strings.Repeat(" ", state.depth * 2),
                match, used, parser.info, string(input))
    return match, used
}

func (parser *Parser) skipWhite(state *state, input[] rune) int {
    space := 0
    if state.skipWhite == 0 {
        for _, char := range input {
            if !unicode.IsSpace(char) {
                break
            }
            space += 1
        }
    }
    return space
}

////////////////////////////////////////////////////////////////////////////////////////////////////

// Set the adjacency flag to true, meaning all subsidiary parsers
// must match with no intervening whitespace.
func (parser *Parser) Adjacent() *Parser {
    parser.adjacent = true
    return parser
}

// Return the passed parser, but with its matching handler bound
// to a callback function.
//
func (parser *Parser) Handle(handler func(s string)) *Parser {
    parser.handler = handler
    return parser
}

// Change the information string of the parser, used during debugging
//
func (parser *Parser) Info(info string) *Parser {
    parser.info = info
    return parser
}

// Parse a string and return results.
//
func (parser *Parser) Parse(input string) {
    parser.invoke(&state{0, 0}, []rune(input))
}
