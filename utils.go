package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

//// Miscellaneous utilities

// Utility fn acting as a ternary
func Ternary[T any](cond bool, r1, r2 T) T {
	if cond {
		return r1
	}
	return r2
}

// Simple fn which takes an integer and maps onto 1 of 5 terminal text colors
// (saving red for error logs) and reset string. Returns ("", "") if NO_COLOR
// set as environment variable on terminal.
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

// Struct on a receiver side of channel implementing io.Writer
type ChannelWriter struct {
	sender chan<- string
}

// io.Writer implementation. Converts to string first because sending to a buffered
// chan []byte was causing unexpected issues when unsynchronized.
func (l ChannelWriter) Write(p []byte) (n int, err error) {
	l.sender <- string(p)
	return 0, nil
}

// Create ChannelWriter from chan<- string
func CreateChannelWriter(sender chan<- string) io.Writer {
	return ChannelWriter{sender: sender}
}

// Fn to be used inside Goroutine. Feeds inputs from send side of channel
// directly into io.Writer
func LogRoutine(writer io.Writer, rcv <-chan string) {
	for log := range rcv {
		writer.Write([]byte(log))
	}
}

// ====Optional

// Option-type implemented without Sum type.
type Optional[T any] struct {
	inner   T
	isEmpty bool
}

// Wraps T into Optional as value of Optional.inner.
// Entrypoint into Optional ecosystem since isEmpty
// cannot be set through other means.
func Some[T any](in T) Optional[T] {
	return Optional[T]{inner: in, isEmpty: false}
}

// For set type T, returns Optional[T] with isEmpty as True.
// Other entrypoint into Optional ecosystem.
func None[T any]() Optional[T] {
	return Optional[T]{isEmpty: true}
}

// Returns whether Optional is not empty.
func (o *Optional[T]) IsSome() bool {
	return !o.isEmpty
}

// Returns whether Optional is empty.
func (o *Optional[T]) IsNone() bool {
	return o.isEmpty
}

// Rust's Option.Unwrap. Dangerous since it may panic.
// Should be preceded by IsNone check or alternatively
// Destructured (error will be non-null if Optional is empty)
func (o *Optional[T]) Unwrap() T {
	if o.IsNone() {
		panic("attempted to unwrap empty Optional!")
	}
	return o.inner
}

// "Fills" Optional with some default value if Optional was empty.
// Returns T value afterwards.
func (o *Optional[T]) UnwrapOr(fallback T) T {
	if o.IsNone() {
		return fallback
	}
	return o.inner
}

// Yields Optional.inner and an error indicating if Optional was empty
func (o *Optional[T]) Destructure() (T, error) {
	if o.IsNone() {
		return o.inner, errors.New("attempted to unwrap empty Optional")
	}
	return o.inner, nil
}

// Yields Optional.inner and Optional.isEmpty
func (o *Optional[T]) DestructureBool() (T, bool) {
	return o.inner, o.isEmpty
}

// Converts non-empty Optional[T] to Ok Result[T]
// Converts empty Optional[T] to Err Result[T]
func (o *Optional[T]) AsResult() Result[T] {
	inner, err := o.Destructure()
	return Result[T]{inner: inner, err: err}
}

// Given Optional[T] and fn: T -> G, returns Optional[fn(T)] if is not empty,
// Otherwise empty Optional[G]
func MapOpt[T, G any](o Optional[T], fn func(T) G) Optional[G] {
	if o.IsNone() {
		return Optional[G]{isEmpty: true}
	}
	return Optional[G]{inner: fn(o.inner), isEmpty: false}
}

// Given Optional[T] and fn: T -> Optional[G], unwraps T and returns fn(T) if
// Optional[T] is not empty, otherwise empty Optional[G]
func FlatMapOpt[T, G any](o Optional[T], fn func(T) Optional[G]) Optional[G] {
	if o.IsNone() {
		return None[G]()
	}
	return fn(o.inner)
}

// ====Result

// Empty struct
// TODO: Embed Result[Empty] as ErrResult to simplify cases where fn just returns error
type Empty struct{}

func NewEmpty() Empty {
	return Empty{}
}

// type ErrResult Result[Empty]
//
// func AsErrResult(err error) ErrResult {
// 	return ErrResult{inner: NewEmpty(), err: err}
// }

// TODO: Add Either(Left | Right) variant

// Result type modelling Ok | Err variants to simplify error handling
type Result[T any] struct {
	inner T
	err   error
}

// Wrap input in Result type as Result's inner
func Ok[T any](in T) Result[T] {
	return Result[T]{inner: in, err: nil}
}

// Wrap error in Result as Result's error
func Err[T any](err error) Result[T] {
	return Result[T]{err: err}
}

// Return if Result is successful/valid
func (r *Result[T]) IsOk() bool {
	return r.err == nil
}

// Return if Result is Error
func (r *Result[T]) IsErr() bool {
	return r.err != nil
}

// Rust's Result.Unwrap. May panic, so should always be preceeded with
// a Result.IsErr block or Destructure should be used instead.
func (r *Result[T]) Unwrap() T {
	if r.IsErr() {
		panic("attempted to unwrap error Result!")
	}
	return r.inner
}

// Yield Result's internal error (may be nil)
func (r *Result[T]) UnwrapErr() error {
	return r.err
}

// Takes an arbitrary T and error and wraps in Result[T]
func Resultify[T any](inner T, err error) Result[T] {
	return Result[T]{inner: inner, err: err}
}

// Opposite of Resultify, undoes Result into (T, error) pair in accordance with
// Go's idiomatic form of error handling
func (r *Result[T]) Destructure() (T, error) {
	return r.inner, r.err
}

// Utility func which drops any inner content of Result and simply
// converts generic type while maintaining the same error
func Transmute[T, G any](r Result[T]) Result[G] {
	return Err[G](r.err)
}

// Given Result[T] and fn: T -> G, returns Result[fn(T)] if Ok
// transmutes Result[T] to Result[G] with same underlying error otherwise
func MapRes[T, G any](r Result[T], fn func(T) G) Result[G] {
	if r.IsErr() {
		return Err[G](r.err)
	}
	return Result[G]{inner: fn(r.inner), err: nil}
}

// Given Result[T] and fn: T -> Result[G], unwraps T (if Ok) and returns fn(T)
// Facilitates chaining of many potentially failing operations in a data pipeline
// by skipping all subsequent applications the moment an Err emerges
func FlatMapRes[T, G any](r Result[T], fn func(T) Result[G]) Result[G] {
	if r.IsErr() {
		return Err[G](r.err)
	}
	return fn(r.inner)
}

// Wraps Result's Error variant in a context string to make robust error handling
// more convenient.
// TODO: Consider changing receiver to value type, copy cost is of int+bool size
// and having to fiddle with deref and the whatnot feels unnecessary if this is
// supposed to be a convneience tool anyways.
func (r *Result[T]) Context(ctx string) *Result[T] {
	if r.IsOk() {
		return r
	}
	r.err = fmt.Errorf(ctx+" (error: %w)", r.err)
	return r
}
