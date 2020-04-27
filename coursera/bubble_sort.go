package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Swap(sli []int, index int) {
	// Swaps the integers  at index and index +1
	temp := sli[index]
	sli[index] = sli[index+1]
	sli[index+1] = temp
}

func BubbleSort(sli []int) {
	// Sort the given slice using the Bubble Sort technique
	sli_len := len(sli)
	i := 0
	for i < sli_len {
		j := 0
		for j < sli_len-1 {
			if sli[j] > sli[j+1] {
				Swap(sli, j)
			}
			j += 1
		}
		i += 1
	}
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter up to 10 integers separated by whitespace")
	text, _ := reader.ReadString('\n')
	sli := make([]int, 10)
	// input_arr := strings.Split(input, []string(" "))
	input_arr := strings.Fields(text)
	for i, item := range input_arr {
		integer, _ := strconv.Atoi(strings.TrimSuffix(item, "\n"))
		sli[i] = integer
	}
	BubbleSort(sli)
	fmt.Println("Sorted slice is ", sli)
}
