package auth_test

import (
	"chirpy/internal/auth"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCheckPasswordHash(t *testing.T) {
	corrects := []struct {
		password string
		hash     string
	}{
		// Empty
		{"", "$2a$10$2IUXpVP61K5Zp/sNYsNwQ..FNP4O1eYkcM/Orry7dYRNkfhw7H4wC"},
		// simple
		{"abc", "$2a$10$IFDDc.mCoiADcZGvbozkp.xGAJteuGAEtqM.r8tmO1x/MMEScC7ru"},
		// emoji and unicode
		{"⚰ 🌻 ♚ 🖋 🏰 💗", "$2a$10$xmbwVFljaS91UM32pkKtQueYjcsgjoABtb6aL2BO8uAqJSJBwonMm"},
	}
	wrongs := []struct {
		password string
		hash     string
	}{
		// Empty
		{"abc", "$2a$10$2IUXpVP61K5Zp/sNYsNwQ..FNP4O1eYkcM/Orry7dYRNkfhw7H4wC"},
		// simple
		{"", "$2a$10$IFDDc.mCoiADcZGvbozkp.xGAJteuGAEtqM.r8tmO1x/MMEScC7ru"},
		// emoji and unicode
		{"⚰ 🌻 ♚ 🖋 🏰", "$2a$10$xmbwVFljaS91UM32pkKtQueYjcsgjoABtb6aL2BO8uAqJSJBwonMm"},
	}

	for _, testcase := range corrects {
		err := auth.CheckPasswordHash(testcase.hash, testcase.password)
		if err != nil {
			t.Errorf("'%s' → '%s' should pass, but doesn't; error: %s",
				testcase.password, testcase.hash, err.Error())
		}
	}

	for _, testcase := range wrongs {
		err := auth.CheckPasswordHash(testcase.hash, testcase.password)
		if err == nil || err.Error() != "password does not match" {
			t.Errorf("'%s' → '%s' should password mismatch, but doesn't; error: %s",
				testcase.password, testcase.hash, err.Error())
		}
	}

	err := auth.CheckPasswordHash("", "")
	if err == nil || !strings.HasPrefix(err.Error(),
		"invalid hash or comparison failed: ") {
		t.Errorf("'' → '' should be invalid, but isn't; error: %s", err.Error())
	}
}

func TestValidateJWT(t *testing.T) {
	// Make a simple JWT, and make sure it decodes
	jwtuuid := uuid.New()
	secret := "secret"
	expiry := time.Minute * 5

	jwt, err := auth.MakeJWT(jwtuuid, secret, expiry)
	if err != nil {
		t.Fatal(err.Error())
	}

	decodedUUID, err := auth.ValidateJWT(jwt, secret)
	if err != nil {
		t.Fatal(err.Error())
	}

	if decodedUUID != jwtuuid {
		t.Logf("generated UUID: %s", jwtuuid.String())
		t.Logf("decoded UUID: %s", decodedUUID.String())
		t.Errorf("generated and decoded UUIDs not equal")
	}
}
