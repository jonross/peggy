package peg

import (
    "reflect"
    "testing"
)

func TestBasics(t *testing.T) {

    letter := AnyOf("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_")
    number := AnyOf("0123456789")
    identifier := Sequence(letter, ZeroOrMoreOf(OneOf(letter, number))).Adjacent().
        Handle(func(info *Info) interface{} { eq(t, "foo", info.Text()); return "got foo" })
    _, _, result := identifier.Parse("foo")
    eq(t, "got foo", result)

    r1 := Literal("->")
    r2 := Literal("->>")
    l1 := Literal("<-")
    l2 := Literal("<<-")

    arrow := OneOf(r1, r2, l1, l2)
    search := Sequence(identifier, ZeroOrMoreOf(Sequence(arrow, identifier)))
    _ = search
}

func eq(t *testing.T, expected interface{}, actual interface{}) {
    vx := reflect.ValueOf(expected)
    va := reflect.ValueOf(actual)
    if vx.Kind() != va.Kind() {
        t.Error("Expected %v but was %v\n", expected, actual)
    }
    switch vx.Kind() {
        case reflect.String:
            xs, _ := expected.(string)
            xa, _ := actual.(string)
            if (xs != xa) {
                t.Errorf("Expected '%s' but was '%s'\n", xs, xa)
            }
        default:
            t.Error("Unhandled kind: %v\n", vx.Kind())
    }
}
