package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	smux := http.NewServeMux()
	srv := http.Server{}
	srv.Addr = ":8080"
	srv.Handler = smux
	err := srv.ListenAndServe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "server error: %s", err.Error())
	}
}
