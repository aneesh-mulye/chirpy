package main

import "chirpy/internal/auth"
import "fmt"
import "os"

func main() {
	if len(os.Args[1:]) != 1 {
		fmt.Fprintln(os.Stderr, "usage: genhash password")
		os.Exit(-1)
	}
	pwd := os.Args[1]
	hashedpw, err := auth.HashPassword(pwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	fmt.Println("\"" + pwd + "\"")
	fmt.Println("→")
	fmt.Println("\"" + hashedpw + "\"")
}
