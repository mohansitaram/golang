package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Animal struct {
	food       string
	locomotion string
	name       string
	noise      string
}

var cow = Animal{"grass", "walk", "cow", "moo"}
var bird = Animal{"worms", "fly", "bird", "peep"}
var snake = Animal{"mice", "slither", "snake", "hsss"}

func (a *Animal) Eat() string {
	return a.food
}

func (a *Animal) Move() string {
	return a.locomotion
}

func (a *Animal) Speak() string {
	return a.noise
}

func PrintInfoForAnimal(animal Animal, info string) {
	switch info {
	case "eat":
		fmt.Println(animal.Eat())
	case "move":
		fmt.Println(animal.Move())
	case "speak":
		fmt.Println(animal.Speak())
	default:
		fmt.Printf("Invalid info request: %s for %s\n", info, animal.name)
	}
}

func GetAnimal(animal string) (Animal, string) {
	switch animal {
	case "cow":
		return cow, ""
	case "bird":
		return bird, ""
	case "snake":
		return snake, ""
	default:
		empty := Animal{}
		err := "The requested animal doesn't exist"
		return empty, err
	}
}

func main() {
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter animal name followed by info separated by space or type 'quit' to quit >")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSuffix(text, "\n")
		if strings.ToLower(text) == "quit" {
			fmt.Println("Quitting")
			return
		}
		request := strings.Split(text, " ")
		if len(request) != 2 {
			fmt.Println("Invalid request format")
			continue
		}
		requested_animal := strings.ToLower(request[0])
		requested_info := strings.ToLower(request[1])
		animal, err := GetAnimal(requested_animal)
		if err != "" {
			fmt.Println(err)
			continue
		}
		PrintInfoForAnimal(animal, requested_info)
	}
}
