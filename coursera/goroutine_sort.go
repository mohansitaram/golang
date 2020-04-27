package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type channel chan []int

func mergeSortedArrays(arr1 []int, arr2 []int) []int {
	// Merges two sorted arrays into one sorted array
	result := make([]int, len(arr1)+len(arr2))
	i, j, k := 0, 0, 0
	for {
		if i >= len(arr1) || j >= len(arr2) {
			break
		} else if arr1[i] <= arr2[j] {
			result[k] = arr1[i]
			i++
		} else {
			result[k] = arr2[j]
			j++
		}
		k++
	}
	for i < len(arr1) {
		result[k] = arr1[i]
		i++
		k++
	}
	for j < len(arr2) {
		result[k] = arr2[j]
		j++
		k++
	}
	return result
}

func readArray() ([]int, string) {
	// Reads the array from stdin
	fmt.Println("Enter an array of integers to sort:")
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	var arr = []int{}
	input_arr := strings.Fields(text)
	for _, item := range input_arr {
		integer, _ := strconv.Atoi(strings.TrimSuffix(item, "\n"))
		arr = append(arr, integer)
	}
	if len(arr) == 0 {
		return arr, "Empty array provided"
	}
	return arr, ""
}

func createSortRoutines(arr []int) []channel {
	// Creates 4 goroutines to sorting the sub arrays
	num_routines := 4
	channels := make([]channel, num_routines)
	slice_begin := 0
	// arr_len := len(arr)
	slice_len := len(arr) / num_routines
	for i := 0; i < num_routines; i++ {
		var sli = []int{}
		if i == 3 {
			// Last goroutine gets the remaining elements
			sli = arr[slice_begin:]
		} else {
			// Else divide equally
			sli = arr[slice_begin : slice_begin+slice_len]
		}
		slice_begin = slice_begin + slice_len
		channels[i] = make(chan []int, len(sli))
		go func(sli []int, c chan []int) {
			if len(sli) == 0 {
				c <- sli
				return
			}
			fmt.Println("This goroutine will sort the slice: ", sli)
			sort.Ints(sli)
			c <- sli
		}(sli, channels[i])
	}
	return channels
}

func merge(channels []channel) {
	// Read sorted arrays from channels
	sorted_slices := make([][]int, len(channels))
	for i, c := range channels {
		sorted_slices[i] = <-c
	}
	result1 := mergeSortedArrays(sorted_slices[0], sorted_slices[1])
	result2 := mergeSortedArrays(sorted_slices[2], sorted_slices[3])
	fmt.Println("Sorted array is ", mergeSortedArrays(result1, result2))
}

func main() {
	arr, err := readArray()
	if err != "" {
		panic(err)
	}
	channels := createSortRoutines(arr)
	merge(channels)
}
