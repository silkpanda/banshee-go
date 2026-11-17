//go:build !windows

package secrets

type unavailableCipher struct{}

// NewOSCipher returns an unavailable cipher on non-Windows platforms.
func NewOSCipher() Cipher {
	return unavailableCipher{}
}

func (unavailableCipher) IsAvailable() bool { return false }

func (unavailableCipher) Encrypt(string) ([]byte, error) {
	return nil, ErrCipherUnavailable
}

func (unavailableCipher) Decrypt([]byte) (string, error) {
	return "", ErrCipherUnavailable
}
