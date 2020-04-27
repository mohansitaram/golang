package main

import (
	"fmt"
	"math/rand"
	"sync"
)

type ChopStick struct{ sync.Mutex }

type Philosopher struct {
	leftChopStick, rightChopStick *ChopStick
	num                           int
	begin_eating                  chan bool
	done_eating                   chan bool
}

var num_philosophers int = 5
var eat_limit int = 3
var wg sync.WaitGroup

func (p Philosopher) eat() {
	for i := 0; i < eat_limit; i++ {
		// Wait for signal from host
		_ = <-p.begin_eating
		p.leftChopStick.Lock()
		p.rightChopStick.Lock()

		fmt.Println("Starting to eat ", p.num)

		p.leftChopStick.Unlock()
		p.rightChopStick.Unlock()
		fmt.Println("Finished eating ", p.num)
		//if i == eat_limit - 1 {
		//    fmt.Println("Finished all turns ", p.num)
		//}
		p.done_eating <- true
	}
}

func host(philosophers []*Philosopher) {
	// Generate a random hosting pattern.
	// This generates 3 random  sequences of philosopher numbers
	defer wg.Done()
	var sequence []int
	for i := 0; i < eat_limit; i++ {
		temp := rand.Perm(num_philosophers)
		for _, j := range temp {
			sequence = append(sequence, j)
		}
	}
	var num_rounds int = num_philosophers * eat_limit
	// Allow only two philosophers to eat at a time
	for i := 0; i < num_rounds; {
		if (num_rounds%2 == 1) && (i+1 >= num_rounds-1) {
			// Corner case: One philosopher will not have a partner because we are allowing only 2 at a time
			p1 := *philosophers[sequence[i]]
			p1.begin_eating <- true
			_ = <-p1.done_eating
			break
		}
		p1, p2 := *philosophers[sequence[i]], *philosophers[sequence[i+1]]
		p1.begin_eating <- true
		p2.begin_eating <- true
		// Wait for both philosphers to finish eating before hosting the next couple
		_ = <-p1.done_eating
		_ = <-p2.done_eating
		i += 2
	}
}

func main() {
	chopSticks := make([]*ChopStick, num_philosophers)
	philosophers := make([]*Philosopher, num_philosophers)
	for i := 0; i < num_philosophers; i++ {
		chopSticks[i] = new(ChopStick)
	}
	for i := 0; i < num_philosophers; i++ {
		begin_eating := make(chan bool)
		done_eating := make(chan bool)
		philosophers[i] = &Philosopher{chopSticks[i], chopSticks[(i+1)%num_philosophers], i + 1, begin_eating, done_eating}
	}
	for i := 0; i < num_philosophers; i++ {
		go philosophers[i].eat()
	}
	wg.Add(1)
	go host(philosophers)
	wg.Wait()
}
