/*
Package peg is a PEG-based parser.
*/
package peggy

import (
    "log"
    "reflect"
    "strconv"
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
    parse func(state *State, input []rune) (bool, int, interface{})
    // if a compound parser, these are subsidiary parsers
    subParsers []*Parser
    // what to call with the string matched by this parser
    handler func(s *State) interface{}
    // do we try to flatten user result arrays
    flatten bool
    // and how far
    howFlat int
    // debug depth when Parse is called
    debug int
}

// Return a Parser that matches any character in a string.
//
func AnyOf(str string) *Parser {
    return newParser("Anyof(" + str + ")", false, nil, func(state *State, input []rune) (bool, int, interface{}) {
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
    return newParser("Literal(" + str + ")", len(str) == 0, nil, func(state *State, input []rune) (bool, int, interface{}) {
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
    }).Handle(func(s *State) interface{} { return s.Text() })
}

// Return a Parser that tries a list of parsers in succession and stops after the
// first that matches.  Arguments may be *Parser or strings; the latter will be
// automatically converted with Literal()
//
func OneOf(pv ...interface{}) *Parser {
    parsers := asParsers(pv)
    return newParser("OneOf", false, parsers, func(state *State, input[]rune) (bool, int, interface{}) {
        for _, parser := range parsers {
            match, used, result := parser.invoke(state, input)
            if match {
                return match, used, result
            }
        }
        return false, 0, nil
    })
}

// Return a Parser that keeps matching as long as any parser in the supplied list
// of parsers matches.  Arguments may be *Parser or strings; the latter will be
// automatically converted with Literal()
//
func ZeroOrMoreOf(pv ...interface{}) *Parser {
    parsers := asParsers(pv)
    return newParser("ZeroOrMoreOf", true, parsers, func(state *State, input[]rune) (bool, int, interface{}) {
        return someOf(false, state, parsers, input)
    })
}

// Like ZeroOrMoreOf but must match at least one, once.
//
func OneOrMoreOf(pv ...interface{}) *Parser {
    parsers := asParsers(pv)
    return newParser("OneOrMoreOf", false, parsers, func(state *State, input[]rune) (bool, int, interface{}) {
        return someOf(true, state, parsers, input)
    })
}

func someOf(mustMatch bool, state *State, parsers []*Parser, input []rune) (bool, int, interface{}) {
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

// Return a parser that optionally matches what another parser matches.  Argument may
// be *Parser or strings; the latter will be automatically converted with Literal()
//
func Optional(p interface{}) *Parser {
    parsers := asParsers([]interface{}{p})
    return newParser("OneOf", true, parsers, func(state *State, input[]rune) (bool, int, interface{}) {
        match, used, result := parsers[0].invoke(state, input)
        if match {
            return match, used, result
        }
        return true, 0, nil
    })
}

// Return a parser that matches if each of the supplied parsers
// matches when tried in succession.
//
func Sequence(pv ...interface{}) *Parser {
    parsers := asParsers(pv)
    return newParser("Sequence", false, parsers, func(state *State, input []rune) (bool, int, interface{}) {
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
               parse func(state *State, input []rune) (bool, int, interface{})) *Parser {
   return &Parser{info, nil, allowEmpty, false, parse, subParsers, nil, false, 0, 0}
}

// Converts untyped array of *Parser / string into []*Parser
//
func asParsers(pv []interface{}) []*Parser {
    parsers := []*Parser{}
    for _, p := range pv {
        parser, ok := p.(*Parser)
        if ok {
            parsers = append(parsers, parser)
        } else {
            str, ok := p.(string)
            if ok {
                parsers = append(parsers, Literal(str))
            } else {
                log.Fatalf("%#v is not a *Parser or string", p)
            }
        }
    }
    return parsers
} 

// Run one pass of a parser.  Skips whitespace if directed, and invokes
// the handler with the string matched.
//
func (parser *Parser) invoke(state *State, input []rune) (bool, int, interface{}) {

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
            state.matched = input[:used]
            state.result = result
            if state.debug > 0 {
                log.Printf("%sHandler => %#v\n", indent(), result)
            }
            result = parser.handler(state)
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

func (parser *Parser) skipWhite(state *State, input[] rune) int {
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
func (parser *Parser) Handle(handler func(s *State) interface{}) *Parser {
    parser.handler = handler
    return parser
}

// Use a Handler that converts the matched text using a predefined or user-defined converter.
// Returns the Parser.
//
func (parser *Parser) As(c Converter) *Parser {
    return parser.Handle(func (s *State) interface{} {
        return c.convert(s)
    })
}

// Change the information string of the parser, used during debugging
//
func (parser *Parser) Describe(text string) *Parser {
    parser.description = text
    return parser
}

// Shortcut for defining a handler that returns the user data object for the indexed parser;
// this is the same as writing
//
//     .Handle(func(s *State) {
//         return s.Get(index).Interface()
//     }
//
func (parser *Parser) Pick(index int) *Parser {
    return parser.Handle(func(s *State) interface{} {
        return s.Get(index).Interface()
    })
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
    return parser.invoke(&State{0, 0, parser.debug, nil, nil}, []rune(input))
}

////////////////////////////////////////////////////////////////////////////////////////////////////

// One of these is created for each call to Parser.Parse; carries state information + is passed
// to user Handlers.
//
type State struct {
    // if == 0 we skip leading whitespace before invoking parser function
    noSkip int
    // recursion depth
    depth int
    // debug depth
    debug int
    // what input was matched
    matched []rune
    // user result returned from parser
    result interface{}
}

// Return the text that was matched by the current Parser.
//
func (s *State) Text() string {
    return string(s.matched)
}

// Returns the length of the user data array, if an array; else
// returns 0.
//
func (s *State) Len() int {
    val := reflect.ValueOf(s.result)
    if val.Kind() == reflect.Slice {
        return val.Len()
    }
    return 0
}

// Return user data associated with the current Parser.  If index==0, return the object as-is;
// if >0 assumes this is a compound parser like a Sequence and return the user data object 
// associated with the (i-1)th parser.
//
func (s *State) Get(index int) reflect.Value {
    val := reflect.ValueOf(s.result)
    if index == 0 {
        return val
    }
    // TODO proper error handling
    return val.Index(index - 1).Elem()
}

////////////////////////////////////////////////////////////////////////////////////////////////////

// Objects implementing this interface may be passed to Parser.As for automatic conversion
// of matched text or sub-parser results
//
type Converter interface {
    convert(s *State) interface{}
}

type FloatConverter int
type IntConverter int
type StringConverter int
type StringsConverter int

func (fc FloatConverter) convert(s *State) interface{} {
    val, _ := strconv.ParseFloat(s.Text(), 64)
    return val // TODO panic if bad
}

func (ic IntConverter) convert(s *State) interface{} {
    val, _ := strconv.ParseInt(s.Text(), 10, 64)
    return val // TODO panic if bad
}

func (sc StringConverter) convert(s *State) interface{} {
    return s.Text()
}

func (sc StringsConverter) convert(s *State) interface{} {
    result := make([]string, s.Len())
    for i, _ := range result {
        result[i] = s.Get(i+1).String()
    }
    return result
}

// A converter that turns matched text into floating-point values
//
const Float = FloatConverter(1)

// A converter that turns matched text into integer values
//
const Int = IntConverter(2)

// A converter that simply returns matched text
//
const String = StringConverter(3)

// A converter that handles when the result array values are all strings
// and turns them into a string slice.
//
const Strings = StringConverter(4)

