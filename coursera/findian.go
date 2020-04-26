package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter a string")
	text, _ := reader.ReadString('\n')                             //  This allows newlines and spaces to be part of input string
	lower_cased := strings.TrimSuffix(strings.ToLower(text), "\n") // Strip trailing newline
	if strings.HasPrefix(lower_cased, "i") == true && strings.HasSuffix(lower_cased, "n") == true && strings.ContainsAny(lower_cased, "a") == true {
		fmt.Println("Found!")
	} else {
		fmt.Println("Not Found!")
	}
}
