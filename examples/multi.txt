package main

import (
	"play.ground/foo"
)

func main() {
	foo.Bar()
}

-- go.mod --
module play.ground

-- foo/foo.go --
package foo

import "fmt"

func Bar() {
	fmt.Println("This function lives in an another file!")
}
