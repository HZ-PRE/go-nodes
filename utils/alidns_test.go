package utils

import "testing"

func TestGuessRootDomain(t *testing.T) {
	for input, want := range map[string]string{
		" WWW.Example.COM. ": "example.com",
		"a.example.co.uk":    "example.co.uk",
		"localhost":          "localhost",
		"":                   "",
		"a..invalid":         ".invalid",
		"a.b..invalid":       ".invalid",
	} {
		if got := guessRootDomain(input); got != want {
			t.Errorf("guessRootDomain(%q) = %q, want %q", input, got, want)
		}
	}
}
