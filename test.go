package main

import (
	"fmt"
	"sort"
)

func sumFn(num ...int) (sum int){
	for _,v := range num {
		sum += v
	}
	return

}

func main() {
	map1 := make(map[int]int)
	map1[10] = 2
	map1[2] = 4
	map1[3] = 5 
	map1[52] = 6
	map1[7] = 9

	var keySlice []int
	for _,v := range map1 {
		keySlice = append(keySlice, v)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(keySlice)))

	fmt.Println(keySlice)

	testSum := sumFn(1,2,3,4,5)
	fmt.Println(testSum)

}
