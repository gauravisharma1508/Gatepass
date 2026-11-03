// Package password wraps bcrypt. The cost is injected so tests can use the minimum.
package password

import (
	"golang.org/x/crypto/bcrypt"
)

type Hasher struct{ Cost int }

func NewHasher(cost int) *Hasher {
	if cost == 0 {
		cost = 12
	}
	return &Hasher{Cost: cost}
}

func (h *Hasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.Cost)
	return string(b), err
}

func (h *Hasher) Check(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
