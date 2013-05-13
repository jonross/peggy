/*
Package peg is a PEG-based parser.
*/
package peg

import (
    "log"
    "reflect"
    "strings"
    "unicode"
)

type Parser struct {
    // for debugging
    description string
    // non-nil if this is only a proxy for another parser
    delegate *Parser
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
    // do we try to flatten user result arrays
    flatten bool
    // and how far
    howFlat int
    // debug depth when Parse is called
    debug int
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
    // debug depth
    debug int
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

// Return a Parser that will match what another parser later specified with Bind() matches.
// TODO: helpful error message if user neglects to call Bind()
//
func Deferred() *Parser {
    return newParser("Proxy", false, nil, nil)
}

// Return a Parser that matches a literal string in the input; also establishes
// a default Handler that returns the text value.
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
    }).Handle(func(info *Info) interface{} { return info.Text() })
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
    return &Parser{info, nil, allowEmpty, false, parse, subParsers, nil, Info{}, false, 0, 0}
}

// Run one pass of a parser.  Skips whitespace if directed, and invokes
// the handler with the string matched.
//
func (parser *Parser) invoke(state *state, input []rune) (bool, int, interface{}) {

    indent := func() string { return strings.Repeat(" ", state.depth * 4) }

    if state.debug > 0 {
        log.Printf("%s-> %s on '%s'\n", indent(), parser.description, string(input))
    }

    state.depth += 1
    state.debug -= 1

    var match bool
    var used int
    var result interface{}

    defer func() {
        state.depth -= 1
        state.debug += 1
        if state.debug > 0 {
            log.Printf("%s<- %s %v, len=%d, result=%v", indent(), parser.description, match, used, result)
        }
    }()

    if (parser.delegate != nil) {
        match, used, result = parser.delegate.invoke(state, input)
        return match, used, result
    }

    // TODO: optimize to skip space once per each change of depth; right now
    // for e.g. OneOf we do it for each subsidiary match attempt
    space := parser.skipWhite(state, input)
    if space > 0 {
        input = input[space:]
    }
    if len(input) == 0 && !parser.allowEmpty {
        return false, 0, nil
    }
    if parser.adjacent {
        state.noSkip += 1
    }

    match, used, result = parser.parse(state, input)

    if match {
        if parser.flatten {
            if reflect.ValueOf(result).Kind() == reflect.Slice {
                if state.debug > 0 {
                    log.Printf("%sflatten -> %#v\n", indent(), result)
                }
                result = flatten(make([]interface{}, 0), result, parser.howFlat + 1)
                if state.debug > 0 {
                    log.Printf("%sflatten <- %#v\n", indent(), result)
                }
            } else {
                if state.debug > 0 {
                    log.Printf("%scan't flatten %#v\n", indent(), result)
                }
            }
        }
        if match && parser.handler != nil {
            parser.context.matched = input[:used]
            parser.context.result = result
            if state.debug > 0 {
                log.Printf("%sHandler => %#v\n", indent(), result)
            }
            result = parser.handler(&parser.context)
            if state.debug > 0 {
                log.Printf("%sHandler <= %#v\n", indent(), result)
            }
        }
    }

    if parser.adjacent {
        state.noSkip -= 1
    }

    return match, used + space, result
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

func flatten(a []interface{}, x interface{}, depth int) []interface{} {
    if depth == 0 {
        a = append(a, x)
        return a
    }
    val := reflect.ValueOf(x)
    if val.Kind() != reflect.Slice {
        a = append(a, x)
        return a
    }
    for i := 0; i < val.Len(); i++ {
        a = flatten(a, val.Index(i).Elem().Interface(), depth - 1)
    }
    return a
}

////////////////////////////////////////////////////////////////////////////////////////////////////

// Set the adjacency flag to true, meaning all subsidiary parsers
// must match with no intervening whitespace.
func (parser *Parser) Adjacent() *Parser {
    parser.adjacent = true
    return parser
}

// Set the debug level; n levels deep of parsers will log details of their execution.  Note this
// applies only to the parser on which Parse() is called.
//
func (parser *Parser) Debug(depth int) *Parser {
    parser.debug = depth
    return parser
}

// Used with a Parser constructed with Deferred() -- specify the parser that will actually run.
//
func (parser *Parser) Bind(delegate *Parser) *Parser {
    parser.delegate = delegate
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
func (parser *Parser) Describe(text string) *Parser {
    parser.description = text
    return parser
}

// Indicate the a parser should flatten N levels of its user result before passing to its
// handler.  This is helpful for dealing with constructs that introduce unneeded nesting.
// Sample Parser setups and possible results with and without flattening:
//
//     Sequence(a, OneOrMoreOf(b))
//     Sequence(a, OneOrMoreOf(b)).Flatten(1)
//
//     [a, [b, b, b]]
//     [a, b, b, b]
//
//  If depth is 0, flatten completely.
//
func (parser *Parser) Flatten(depth int) *Parser {
    parser.flatten = true
    parser.howFlat = depth
    return parser
}

// Parse a string and return results.
//
func (parser *Parser) Parse(input string) (bool, int, interface{}) {
    return parser.invoke(&state{0, 0, parser.debug}, []rune(input))
}

////////////////////////////////////////////////////////////////////////////////////////////////////

// Return the text that was matched by the current Parser.
//
func (info *Info) Text() string {
    return string(info.matched)
}

// Returns the length of the user data array, if an array; else
// returns 0.
//
func (info *Info) Len() int {
    val := reflect.ValueOf(info.result)
    if val.Kind() == reflect.Slice {
        return val.Len()
    }
    return 0
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
    return val.Index(index - 1).Elem()
}

