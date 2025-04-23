package identifiers

import (
	"fmt"
	"math/rand"
	"time"
)

func Hash() string {
	timestamp := time.Now().Unix()

	rand.Seed(time.Now().UnixNano())
	randomNumber := rand.Int()

	hash := fmt.Sprintf("%d-%d", timestamp, randomNumber)

	return hash
}
