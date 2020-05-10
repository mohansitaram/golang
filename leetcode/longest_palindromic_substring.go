package main

import "fmt"

func longestPalindrome(s string) string {
	var len_s int = len(s)
	if len_s == 0 {
		return s
	}
	// Create a N * N table
	var table = make([][]bool, len_s)
	for i := range table {
		table[i] = make([]bool, len_s)
	}
	// All strings of length 1 are palindromes
	for i := 0; i < len_s; i++ {
		table[i][i] = true
	}
	// Check for all substrings of length 2. This is to explicitly
	// handle cases where adjacent characters are identical which is still
	// a valid palindrome
        // max_length is set to 1 for the default case of no palindromic substring
	var max_length, start int = 1, 0
	for i := 0; i < len_s-1; i++ {
		if s[i] == s[i+1] {
			table[i][i+1] = true
			start = i
			max_length = 2
		}
	}

	// Check for all substrings of length >= 3
	for k := 3; k <= len_s; k++ {
		for i := 0; i <= len_s-k; i++ {
			j := i + k - 1
			// This checks if a substring is a palindrome based on whether or not
			// its next smallest substring is a palindrome.
			if table[i+1][j-1] == true && s[i] == s[j] {
				table[i][j] = true
				if k > max_length {
					max_length = k
					start = i
				}
			}
		}
	}
	for _, row := range table {
		fmt.Println(row)
	}
	return s[start : start+max_length]
}

func main() {
	s := "cbbd"
	result := longestPalindrome(s)
	fmt.Println("Result is ", result)
}
