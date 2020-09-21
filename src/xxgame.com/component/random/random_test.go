package random

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestRandom(t *testing.T) {
	library := make(map[string]bool, 10)
	conflicts := 0
	loop := 1000000
	for {
		s := hex.EncodeToString(GetRandomBytes(3))
		if library[s] {
			conflicts++
		} else {
			library[s] = true
		}
		loop--
		if loop <= 0 {
			break
		}
	}

	fmt.Printf("conflicts: %d\n", conflicts)
}
