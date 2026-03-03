package names

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Generate returns a random friendly name in the format "adjective_noun".
func Generate() string {
	l := randItem(left)
	r := randItem(right)
	return l + "_" + r
}

// GenerateUnique returns a name that does not collide according to the exists function.
// It tries the base name, then appends _2, _3 ... _99. If all collide, it generates
// a new base name (up to 10 rounds). Ultimate fallback: "name_" + 6 random hex chars.
func GenerateUnique(exists func(string) bool) string {
	for round := 0; round < 10; round++ {
		base := Generate()
		if !exists(base) {
			return base
		}
		for suffix := 2; suffix <= 99; suffix++ {
			candidate := fmt.Sprintf("%s_%d", base, suffix)
			if !exists(candidate) {
				return candidate
			}
		}
	}
	// Fallback: random hex
	return "name_" + randHex(6)
}

// randItem picks a random element from a slice using crypto/rand.
func randItem(items []string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(items))))
	if err != nil {
		// Fallback to first item on entropy failure (extremely unlikely).
		return items[0]
	}
	return items[n.Int64()]
}

// randHex returns n random hex characters.
func randHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return fmt.Sprintf("%x", b)[:n]
}
