package identifiers

import (
	"fmt"
	"math/rand"
	"time"
)

// Hash generates a unique container identifier
// Format: <unix_timestamp>-<random_number>
// Example: 1742583600-123456789
// Timestamp ensures chronological ordering, random number prevents collisions
func Hash() string {
	timestamp := time.Now().Unix()

	rand.Seed(time.Now().UnixNano())
	randomNumber := rand.Int()

	hash := fmt.Sprintf("%d-%d", timestamp, randomNumber)

	return hash
}
