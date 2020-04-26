package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	var info = make(map[string]string)
	// using bufio to accept whole strings as input including whitespaces
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter name:")
	info["name"], _ = reader.ReadString('\n')
	fmt.Println("Enter address:")
	info["address"], _ = reader.ReadString('\n')
	for k, v := range info {
		info[k] = strings.TrimSuffix(v, "\n")
	}
	json_info, _ := json.Marshal(info)
	fmt.Println("JSON object is ", string(json_info))
}
