package peggy

import (
    . "launchpad.net/gocheck"
    "log"
    "os"
    "testing"
)

// Hook up gocheck into the "go test" runner. 
func Test(t *testing.T) { TestingT(t) }
type MySuite struct{} 
var _ = Suite(&MySuite{})

type TypeVar struct {
    arrow *string
    typeSpec *string
    varName *string
}

// This fixture still in progress, has some test code for helmet.
//
func (s *MySuite) TestBasics(c *C) {

    letter := AnyOf("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_")
    number := AnyOf("0123456789")

    identifier := Sequence(letter, ZeroOrMoreOf(OneOf(letter, number))).Adjacent().As(String)

    _, _, result := identifier.Parse("foo")
    c.Check("foo", Equals, result)

    typeVar := Sequence(identifier, identifier).
        Handle(func(s *State) interface{} {
            typeSpec := s.Get(1).String()
            varName := s.Get(2).String()
            return &TypeVar{nil, &typeSpec, &varName}
        })

    r1 := Literal("->")
    r2 := Literal("->>")
    l1 := Literal("<-")
    l2 := Literal("<<-")

    arrow := OneOf(r1, r2, l1, l2)
    step := Sequence(arrow, typeVar).
        Handle(func(s *State) interface{} {
            arr := s.Get(1).String()
            tv := s.Get(2).Interface().(*TypeVar)
            tv.arrow = &arr
            return tv
        })

    search := Sequence(typeVar, ZeroOrMoreOf(step))
    search.Parse("a b -> c d ->> e f")
}

// Simple calculator.  User data values are simply floats.
//
func (s *MySuite) TestCalculator(c *C) {

    console, err := os.OpenFile("./test.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
    if err != nil {
        log.Fatalln(err)
    }
    log.SetOutput(console)

    // Define parsers for the following EBNF:
    //
    // expr1 := expr2 [ ( "+" | "-" ) expr2 ]
    // expr2 := expr3 [ ( "*" | "/" ) expr3 ]
    // expr3 := number | "(" expr1 ")"
    // number := { digit } | [ digit ] "." { digit }
    // digit := "0" ... "9"

    digits := OneOrMoreOf(AnyOf("0123456789"))
    number := OneOf(digits, Sequence(Optional(digits), ".", digits)).Adjacent().As(Float)

    makeOp := func(s *State) interface{} {
        op := s.Get(1).String()
        rhs := s.Get(2).Float()
        return func(lhs float64) float64 { 
            switch op {
                case "+": return lhs + rhs
                case "-": return lhs - rhs
                case "*": return lhs * rhs
                case "/": return lhs / rhs
                default: panic("bad op: " + op)
            }
        }
    }

    evalOps := func(s *State) interface{} {
        val := s.Get(1).Float()
        for i := 1; i < s.Len(); i++ {
            fn := s.Get(i + 1).Interface().(func(float64) float64)
            val = fn(val)
        }
        return val
    }

    expr1 := Deferred()
    expr3 := OneOf(number, Sequence("(", expr1, ")").Pick(2)).Describe("expr3")

    mulOps := ZeroOrMoreOf(Sequence(OneOf("*", "/"), expr3).Handle(makeOp)).Describe("mulops")
    expr2 := Sequence(expr3, mulOps).Flatten(1).Describe("expr2").Handle(evalOps)

    addOps := ZeroOrMoreOf(Sequence(OneOf("+", "-"), expr2).Handle(makeOp)).Describe("addops")
    _xpr1 := Sequence(expr2, addOps).Flatten(1).Describe("expr1").Handle(evalOps)

    expr1.Bind(_xpr1).Debug(4)

    try := func(expr string, expected float64) {
        _, _, result := expr1.Parse(expr)
        actual := result.(float64)
        c.Check(actual, Equals, expected)
    }

    try(" 1 + 2  + 3", 6.0)
    try(" 1 + 2  * 3", 7.0)
    try("(1 + 2) * 3", 9.0)
    try("(1 + 2) / 3", 1.0)

    try ("5 + (5 * 5) - ((5 + 5) / 5)", 28.0)
}

