package main

import (
	"chirpy/internal/auth"
	"fmt"
	"os"
)

func main() {
	if len(os.Args[1:]) != 2 {
		fmt.Fprintln(os.Stderr, "usage: checkpwdhash hash password")
		os.Exit(-1)
	}
	hashstring := os.Args[1]
	password := os.Args[2]
	err := auth.CheckPasswordHash(hashstring, password)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(-1)
	}
	fmt.Println("match")
}
