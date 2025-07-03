package main

import (
	"fmt"
	"os"

	"chirpy/internal/auth"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "invalid number of arguments"+"\n"+
			"usage: <validateJTW> jwt secret")
		os.Exit(-1)
	}

	jwt := os.Args[1]
	secret := os.Args[2]

	uuid, err := auth.ValidateJWT(jwt, secret)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error decoding token: "+err.Error())
		os.Exit(-1)
	}

	fmt.Println("UUID: " + uuid.String())
}
