/*
Package peg is a PEG-based parser.
*/
package peg

import (
    "fmt"
    "reflect"
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
    // actual parse function, returns whether matched + amount of input consumed + user result
    parse func(state *state, input []rune) (bool, int, interface{})
    // if a compound parser, these are subsidiary parsers
    subParsers []*Parser
    // what to call with the string matched by this parser
    handler func(info *Info) interface{}
    // what to pass to the handler
    context Info
}

// This is passed to user callbacks.  Parser field is private because we don't want the user
// modifying the parser during a parse.
//
type Info struct {
    parser *Parser
    // what input was matched
    matched []rune
    // user result returned from parser
    result interface{}
}

type state struct {
    // if == 0 we skip leading whitespace before invoking parser function
    noSkip int
    // recursion depth
    depth int
}

// Return a Parser that matches any character in a string.
//
func AnyOf(str string) *Parser {
    return newParser("Anyof(" + str + ")", false, nil, func(state *state, input []rune) (bool, int, interface{}) {
        // TODO: optimize
        for _, char := range str {
            if input[0] == char {
                return true, 1, nil
            }
        }
        return false, 0, nil
    })
}

// Return a Parser that matches a literal string in the input.
//
func Literal(str string) *Parser {
    runes := []rune(str)
    strLen := len(runes)
    return newParser("Literal(" + str + ")", len(str) == 0, nil, func(state *state, input []rune) (bool, int, interface{}) {
        inputLen := len(input)
        if strLen > inputLen {
            return false, 0, nil
        }
        for i, char := range runes {
            if char != input[i] {
                return false, 0, nil
            }
        }
        return true, strLen, nil
    })
}

// Return a Parser that tries a list of parsers in succession
// and stops after the first that matches.
//
func OneOf(parsers ...*Parser) *Parser {
    return newParser("OneOf", false, parsers, func(state *state, input[]rune) (bool, int, interface{}) {
        for _, parser := range parsers {
            match, used, result := parser.invoke(state, input)
            if match {
                return match, used, result
            }
        }
        return false, 0, nil
    })
}

// Return a Parser that keeps matching as long as any parser
// in the supplied list of parsers matches.
//
func ZeroOrMoreOf(parsers ...*Parser) *Parser {
    return newParser("ZeroOrMoreOf", true, parsers, func(state *state, input[]rune) (bool, int, interface{}) {
        return someOf(false, state, parsers, input)
    })
}

// Like ZeroOrMoreOf but must match at least one, once.
//
func OneOrMoreOf(parsers ...*Parser) *Parser {
    return newParser("OneOrMoreOf", false, parsers, func(state *state, input[]rune) (bool, int, interface{}) {
        return someOf(true, state, parsers, input)
    })
}

func someOf(mustMatch bool, state *state, parsers []*Parser, input []rune) (bool, int, interface{}) {
    totalUsed := 0
    hasMatched := false
    results := make([]interface{}, 0)
    for {
        for _, parser := range parsers {
            match, used, result :=  parser.invoke(state, input)
            if match {
                totalUsed += used
                input = input[used:]
                results = append(results, result)
                hasMatched = true
                break
            } else if mustMatch && !hasMatched {
                return false, 0, nil
            } else {
                return true, totalUsed, results
            }
        }
    }
}

// Return a parser that optionally matches what another parser matches.
//

func Optional(parser *Parser) *Parser {
    return newParser("OneOf", true, []*Parser{parser}, func(state *state, input[]rune) (bool, int, interface{}) {
        match, used, result := parser.invoke(state, input)
        if match {
            return match, used, result
        }
        return true, 0, nil
    })
}

// Return a parser that matches if each of the supplied parsers
// matches when tried in succession.
//
func Sequence(parsers ...*Parser) *Parser {
    return newParser("Sequence", false, parsers, func(state *state, input []rune) (bool, int, interface{}) {
        totalUsed := 0
        results := make([]interface{}, 0)
        for _, parser := range parsers {
            match, used, result := parser.invoke(state, input)
            if ! match {
                return false, 0, nil
            }
            totalUsed += used
            input = input[used:]
            results = append(results, result)
        }
        return true, totalUsed, results
    })
}

// Creates a Parser node around a parsing function.
//
func newParser(info string, allowEmpty bool, subParsers []*Parser, 
               parse func(state *state, input []rune) (bool, int, interface{})) *Parser {
    return &Parser{info, allowEmpty, false, parse, subParsers, nil, Info{}}
}

// Run one pass of a parser.  Skips whitespace if directed, and invokes
// the handler with the string matched.
//
func (parser *Parser) invoke(state *state, input []rune) (bool, int, interface{}) {
    fmt.Printf("%s-> %s on '%s'\n", strings.Repeat(" ", state.depth * 2), 
                parser.info, string(input))
    // TODO: optimize to skip space once per each change of depth; right now
    // for e.g. OneOf we do it for each subsidiary match attempt
    space := parser.skipWhite(state, input)
    if space > 0 {
        input = input[space:]
    }
    if len(input) == 0 && !parser.allowEmpty {
        return false, 0, nil
    }
    state.depth += 1
    if parser.adjacent {
        state.noSkip += 1
    }
    match, used, result := parser.parse(state, input)
    if match && parser.handler != nil {
        parser.context.matched = input[:used]
        parser.context.result = result
        result = parser.handler(&parser.context)
    }
    state.depth -= 1
    if parser.adjacent {
        state.noSkip -= 1
    }
    fmt.Printf("%s%v, %d <- %s on '%s'\n", strings.Repeat(" ", state.depth * 2),
                match, used, parser.info, string(input))
    return match, used, result
}

func (parser *Parser) skipWhite(state *state, input[] rune) int {
    space := 0
    if state.noSkip == 0 {
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
func (parser *Parser) Handle(handler func(info *Info) interface{}) *Parser {
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
func (parser *Parser) Parse(input string) (bool, int, interface{}) {
    return parser.invoke(&state{0, 0}, []rune(input))
}

////////////////////////////////////////////////////////////////////////////////////////////////////

// Return the text that was matched by the current Parser.
//
func (info *Info) Text() string {
    return string(info.matched)
}

// Return user data associated with the current Parser.  If index==0,
// return the object as-is, if >0 assumes this is a compound parser
// and return the user data object associated with the (i-1)th parser.
//
func (info *Info) Get(index int) reflect.Value {
    val := reflect.ValueOf(info.result)
    if index == 0 {
        return val
    }
    // TODO proper error handling
    return val.Index(index - 1)
}

