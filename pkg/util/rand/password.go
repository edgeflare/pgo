package rand

import (
	"crypto/rand"
	"math/big"
)

// NewPassword generates a cryptographically secure random password.
//
// The password consists of a mix of lowercase letters, uppercase letters,
// numbers, and symbols. The default length is 16 characters. You can optionally
// provide a `length` argument to specify a different length.
//
// If the provided length is less than or equal to 0, the default length of 16
// is used.
//
// This function uses the `crypto/rand` package to ensure a high level of
// randomness and security, suitable for generating passwords
func NewPassword(length ...int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"
	passwordLength := 16 // Default length

	if len(length) > 0 {
		passwordLength = length[0]
		if passwordLength <= 0 {
			passwordLength = 16 // Reset to default if invalid length is provided
		}
	}

	// Use crypto/rand for a cryptographically secure random number generator
	random := rand.Reader

	b := make([]byte, passwordLength)
	for i := range b {
		num, err := rand.Int(random, big.NewInt(int64(len(charset))))
		if err != nil {
			// Handle the error appropriately (e.g., panic, log, retry)
			panic(err)
		}
		b[i] = charset[num.Int64()]
	}
	return string(b)
}
