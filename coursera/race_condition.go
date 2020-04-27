package main

import (
	"fmt"
	"time"
)

func foo(x *int) {
	*x = *x + 1
	fmt.Println("Value of shared variable in foo is ", *x)
	time.Sleep(1000 * time.Millisecond)
	*x = *x + 2
	fmt.Println("Value of shared variable in foo is ", *x)
}

func bar(x *int) {
	*x = *x - 1
	time.Sleep(1000 * time.Millisecond)
	fmt.Println("Value of shared variable in bar is ", *x)
	*x = *x - 2
	fmt.Println("Value of shared variable in bar is ", *x)
}

func main() {
	var shared_var int = 0
	go foo(&shared_var)
	go bar(&shared_var)
	// The Sleeps here and elsewhere arerequired to give the goroutines a chance to
	// run. This hack can be replaced by using channels but the course is not there yet.
	time.Sleep(1000 * time.Millisecond)
	fmt.Println("Final value of shared variable is ", shared_var)
}
