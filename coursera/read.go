package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
)

type Name struct {
	fname string
	lname string
}

func main() {
	var filename string
	fmt.Println("Enter the name of the file")
	fmt.Scan(&filename)
	data := make([]byte, 0)
	data, _ = ioutil.ReadFile(filename)
	str_names := bytes.Split(data, []byte("\n"))
	// EOF is also considered valid so stripping it oof below
	str_names = str_names[:len(str_names)-1]
	// Initialize slice of structs. We know the size
	names := make([]Name, len(str_names))
	for i, name := range str_names {
		split_name := bytes.Split(name, []byte(" "))
		var fname = string(split_name[0])
		var lname = string(split_name[1])
		names[i] = Name{fname, lname}
	}
	// Print the names
	for _, name := range names {
		fmt.Printf("First name: %s, Last name: %s\n", name.fname, name.lname)
	}
}
