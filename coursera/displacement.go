package main

import (
    "fmt"
    "math"
)


func GenDisplaceFn(accl float64, velocity float64, initial_disp float64) func (time float64) float64 {
    return func(time float64) float64 {
        var disp = (0.5 * accl * math.Pow(time, 2)) + (velocity * time) + initial_disp
        return disp
    }
}

func main() {
    var accl, velocity, initial_disp, time float64
    fmt.Println("Enter accelaration")
    fmt.Scan(&accl)
    fmt.Println("Enter velocity")
    fmt.Scan(&velocity)
    fmt.Println("Enter initial displacement")
    fmt.Scan(&initial_disp)
    fmt.Println("Enter the time")
    fmt.Scan(&time)
    fn := GenDisplaceFn(accl, velocity, initial_disp)
    fmt.Printf("Displacement after time %f is ", time)
    fmt.Println(fn(time))
}
