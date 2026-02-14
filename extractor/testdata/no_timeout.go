package sample

import "fmt"

// Hello prints a greeting. No timeout or retry configuration here.
func Hello() {
	fmt.Println("hello world")
}

func Add(a, b int) int {
	return a + b
}
