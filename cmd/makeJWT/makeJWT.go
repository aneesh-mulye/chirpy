package main

// Don't take a UUID, just something else?
/*
What's the testing strategy in general for this?
How can I test the MakeJWT function?
And what do I want to be able to do here to generate the test cases?

First, the ability to just *call* it, with an option to have it generate the
UUID itself if I need that.
Then, some way to try to make invalid tokens? So something which has already
expired; so, offset? Or just have a token that is set to expire one second after
the 'now' at the time the command is run, so that in any tests, it's going to
def be expired? Is that better?

OK, what's the strategy? Let's go pathwise.

Actually, not sure there's much point to testing this in isolation. Much better
to test in pairs, and focus the bulk of the effort on the ValidateJWT function.

So the test cases will *not* have 'issuer' set separately, since it's always
implicitly set in the functions. They'll only have a UUID, a secret, and prob
that's it? Since the expiresIn is something I will decide within the cases?

OK, so what cases, then?

1) Straight line case, where you generate a thing that's correct, and that's it,
   you're done.
2) Expiry stuff.
3) Where the UUID isn't valid, so the library can't parse the Subject claim as
   a UUID.

Anyway, this later. TODO

Now, want to make this simple think to make the JWT.
*/

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"chirpy/internal/auth"

	"github.com/google/uuid"
)

func main() {
	// Parse arguments
	if len(os.Args) < 3 || len(os.Args) > 4 {
		fmt.Fprintln(os.Stderr,
			"wrong argument arity;\nusage: <makeJWT> UUID secret [expiresIn]")
		os.Exit(-1)
	}
	var jwtUUID uuid.UUID
	if os.Args[1] == "gen" {
		jwtUUID = uuid.New()
	} else {
		var err error
		jwtUUID, err = uuid.Parse(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid UUID: %s\n%s", os.Args[1], err.Error())
		}
	}

	secret := os.Args[2]

	expiresInMinutes := 5
	if len(os.Args) == 4 {
		var err error
		expiresInMinutes, err = strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid expiry time: "+err.Error())
		}
	}
	expiresIn := time.Minute * time.Duration(expiresInMinutes)

	jwt, err := auth.MakeJWT(jwtUUID, secret, expiresIn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(-1)
	}

	fmt.Println("UUID: " + jwtUUID.String())
	fmt.Println("secret: " + secret)
	fmt.Println("expires in: " + strconv.Itoa(expiresInMinutes) + "m")
	fmt.Println("→")
	fmt.Println("JWT: '" + jwt + "'")
}
