package infrastructure

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher hashes passwords with bcrypt. Cost must be >= 12 in production
// per MVP-001 general rules; tests may inject a lower cost.
type BcryptHasher struct {
	Cost int
}

// NewBcryptHasher builds a hasher with the given cost.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{Cost: cost}
}

// Hash returns the bcrypt hash.
func (h *BcryptHasher) Hash(password string) (string, error) {
	sum, err := bcrypt.GenerateFromPassword([]byte(password), h.Cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(sum), nil
}

// Compare verifies a password against its hash.
func (h *BcryptHasher) Compare(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("bcrypt compare: %w", err)
	}
	return nil
}
