package p_test

// This file defines various Test, Benchmark, Example, and Fuzz
// functions for use by TestIsTest in sandbox_test.go. Some are
// ill-formed and will fail go vet's static checks, would fail at
// runtime if executed by go test, or would not be recognized as tests
// by either tool.

import "testing"

// TestisNotATest is not a test function, despite appearances.
//
// Please ignore any lint or vet warnings for this function.
func TestisNotATest(t *testing.T) {
	panic("This is not a valid test function.")
}

// Test1IsATest is a valid test function.
func Test1IsATest(t *testing.T) {
}

// Test is a test with a minimal name.
func Test(t *testing.T) {
}

// TestÑIsATest is a test with an interesting Unicode name.
func TestÑIsATest(t *testing.T) {
}

func Example() {
	// Output:
}

func ExampleTest() {
	// This is an example for the function Test.
	// ❤ recursion.
	Test(nil)

	// Output:
}

func Example1IsAnExample() {
	// Output:
}

// ExampleisNotAnExample is not an example function, despite appearances.
//
// Please ignore any lint or vet warnings for this function.
func ExampleisNotAnExample() {
	panic("This is not a valid example function.")

	// Output:
	// None. (This is not really an example function.)
}

func Example_isAnExample() {
	// Output:
}

func ExampleTest_isAnExample() {
	Test(nil)

	// Output:
}

func Example_noOutput() {
	// No output declared: should be compiled but not run.
}

func Benchmark(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

func BenchmarkNop(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

func Benchmark1IsABenchmark(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

// BenchmarkisNotABenchmark is not a benchmark function, despite appearances.
//
// Please ignore any lint or vet warnings for this function.
func BenchmarkisNotABenchmark(b *testing.B) {
	panic("This is not a valid benchmark function.")
}

// FuzzisNotAFuzz is not a fuzz test function, despite appearances.
//
// Please ignore any lint or vet warnings for this function.
func FuzzisNotAFuzz(f *testing.F) {
	panic("This is not a valid fuzzing function.")
}

// Fuzz1IsAFuzz is a valid fuzz function.
func Fuzz1IsAFuzz(f *testing.F) {
	f.Skip()
}

// Fuzz is a fuzz with a minimal name.
func Fuzz(f *testing.F) {
	f.Skip()
}

// FuzzÑIsAFuzz is a fuzz with an interesting Unicode name.
func FuzzÑIsAFuzz(f *testing.F) {
	f.Skip()
}
