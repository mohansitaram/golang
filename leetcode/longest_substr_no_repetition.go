package main

import (
    "fmt"
    "strings"
)

func lengthOfLongestSubstring(s string) int {
    if len(s) == 0 || len(s) == 1 {
        return len(s)
    }
    var start, end int
    var running_start int = 0
    var i int = 1
    for ; i < len(s); i++ {
        index := strings.Index(s[running_start:i], string(s[i]))
        if index != -1 {
            if (i - running_start) > (end - start) {
                end = i
                start = running_start
            }
            // Reset running substring
            running_start += index + 1
        }
    }
    if (i - running_start) > (end - start) {
        end = i
        start = running_start
    }
    fmt.Println("Longest substring is ", s[start:end])
    return len(s[start:end])
}

func main() {
    var input string = "abcdabcbb"
    res := lengthOfLongestSubstring(input)
    fmt.Println("Result is", res)
}
