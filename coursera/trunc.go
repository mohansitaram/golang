package main

import "fmt"

func main() {
        var float_num float64
	fmt.Println("Enter a floating point number")
	fmt.Scan(&float_num)
	fmt.Println("The float truncated to int is", int(float_num))
}
