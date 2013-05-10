package peg

import (
    . "launchpad.net/gocheck"
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

func (s *MySuite) TestBasics(c *C) {

    letter := AnyOf("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_")
    number := AnyOf("0123456789")

    identifier := Sequence(letter, ZeroOrMoreOf(OneOf(letter, number))).Adjacent().
        Handle(func(info *Info) interface{} { 
            return info.Text()
        })

    _, _, result := identifier.Parse("foo")
    c.Check("foo", Equals, result)

    typeVar := Sequence(identifier, identifier).
        Handle(func(info *Info) interface{} {
            typeSpec := info.Get(1).String()
            var varName string
            if info.Get(2).IsNil() {
                return &TypeVar{nil, &typeSpec, nil}
            }
            varName = info.Get(2).String()
            return &TypeVar{nil, &typeSpec, &varName}
        })

    r1 := Literal("->")
    r2 := Literal("->>")
    l1 := Literal("<-")
    l2 := Literal("<<-")

    arrow := OneOf(r1, r2, l1, l2)
    step := Sequence(arrow, typeVar).
        Handle(func(info *Info) interface{} {
            arr := info.Get(1).String()
            tv := info.Get(2).Interface().(*TypeVar)
            tv.arrow = &arr
            return tv
        })

    search := Sequence(typeVar, ZeroOrMoreOf(step))
    search.Parse("a b -> c d ->> e f")
}
