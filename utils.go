package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

type ChannelWriter struct {
	sender chan<- string
}

func (l ChannelWriter) Write(p []byte) (n int, err error) {
	l.sender <- string(p)
	return 0, nil
}

func CreateChannelWriter(sender chan<- string) io.Writer {
	return ChannelWriter{sender: sender}
}

func LogRoutine(writer io.Writer, rcv <-chan string) {
	for log := range rcv {
		writer.Write([]byte(log))
	}
}

type Optional[T any] struct {
	inner   T
	isEmpty bool
}

func Some[T any](in T) Optional[T] {
	return Optional[T]{inner: in, isEmpty: false}
}

func None[T any]() Optional[T] {
	return Optional[T]{isEmpty: true}
}

func (o *Optional[T]) IsSome() bool {
	return !o.isEmpty
}

func (o *Optional[T]) IsNone() bool {
	return o.isEmpty
}

func (o *Optional[T]) Unwrap() T {
	if o.IsNone() {
		panic("attempted to unwrap empty Optional!")
	}
	return o.inner
}

func (o *Optional[T]) UnwrapOr(fallback T) T {
	if o.IsNone() {
		return fallback
	}
	return o.inner
}

func (o *Optional[T]) Destructure() (T, error) {
	if o.IsNone() {
		return o.inner, errors.New("attempted to unwrap empty Optional")
	}
	return o.inner, nil
}

func MapOpt[T, G any](o Optional[T], fn func(T) G) Optional[G] {
	if o.IsNone() {
		return Optional[G]{isEmpty: true}
	}
	return Optional[G]{inner: fn(o.inner), isEmpty: false}
}

func FlatMapOpt[T, G any](o Optional[T], fn func(T) Optional[G]) Optional[G] {
	if o.IsNone() {
		return None[G]()
	}
	return fn(o.inner)
}

// ====Result

type Empty struct{}

func NewEmpty() Empty {
	return Empty{}
}

type ErrResult Result[Empty]

func AsErrResult(err error) ErrResult {
	return ErrResult{inner: NewEmpty(), err: err}
}

// TODO: Add Either(Left | Right) variant
type Result[T any] struct {
	inner T
	err   error
}

func Ok[T any](in T) Result[T] {
	return Result[T]{inner: in, err: nil}
}

func Err[T any](err error) Result[T] {
	return Result[T]{err: err}
}

func (r *Result[T]) IsOk() bool {
	return r.err == nil
}

func (r *Result[T]) IsErr() bool {
	return r.err != nil
}

func (r *Result[T]) Unwrap() T {
	if r.IsErr() {
		panic("attempted to unwrap error Result!")
	}
	return r.inner
}

func (r *Result[T]) UnwrapErr() error {
	return r.err
}

func (r *Result[T]) Destructure() (T, error) {
	return r.inner, r.err
}

func Transmute[T, G any](r Result[T]) Result[G] {
	return Err[G](r.err)
}

func MapRes[T, G any](r Result[T], fn func(T) G) Result[G] {
	if r.IsErr() {
		return Result[G]{err: r.err}
	}
	return Result[G]{inner: fn(r.inner), err: nil}
}

func FlatMapRes[T, G any](r Result[T], fn func(T) Result[G]) Result[G] {
	if r.IsErr() {
		return Err[G](r.err)
	}
	return fn(r.inner)
}

func (r *Result[T]) Context(ctx string) *Result[T] {
	if r.IsOk() {
		return r
	}
	r.err = fmt.Errorf(ctx+" (error: %w)", r.err)
	return r
}

func Ternary[T any](cond bool, r1, r2 T) T {
	if cond {
		return r1
	}
	return r2
}

func TerminalColors(n int) (string, string) {
	if len(os.Getenv("NO_COLOR")) != 0 {
		return "", ""
	}
	ColorMap := map[int]string{
		0: "\033[0m",  // Reset
		1: "\033[31m", // Red
		2: "\033[32m", // Green
		3: "\033[33m", // Yellow
		4: "\033[34m", // Blue
		5: "\033[35m", // Magenta
		6: "\033[36m", // Cyan
	}
	AvailColors := len(ColorMap) - 2
	color_choice := n%AvailColors + 2
	return ColorMap[color_choice], ColorMap[0]
}
