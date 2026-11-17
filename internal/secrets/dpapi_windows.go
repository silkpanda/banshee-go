//go:build windows

package secrets

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPICipher encrypts and decrypts secrets using the current Windows user's DPAPI credentials.
type DPAPICipher struct {
	available bool
}

// NewOSCipher probes Windows DPAPI availability and returns the active cipher.
func NewOSCipher() Cipher {
	c := &DPAPICipher{available: true}
	blob, err := c.Encrypt("banshee-dpapi-probe")
	if err != nil {
		return &DPAPICipher{available: false}
	}
	plain, err := c.Decrypt(blob)
	if err != nil || plain != "banshee-dpapi-probe" {
		return &DPAPICipher{available: false}
	}
	return c
}

func (c *DPAPICipher) IsAvailable() bool {
	return c.available
}

func (c *DPAPICipher) Encrypt(plaintext string) ([]byte, error) {
	if !c.available {
		return nil, ErrCipherUnavailable
	}
	raw := []byte(plaintext)
	var inBlob windows.DataBlob
	if len(raw) > 0 {
		inBlob = windows.DataBlob{
			Size: uint32(len(raw)),
			Data: &raw[0],
		}
	}
	var outBlob windows.DataBlob
	err := windows.CryptProtectData(
		&inBlob,
		nil,
		nil,
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&outBlob,
	)
	if err != nil {
		return nil, fmt.Errorf("dpapi encrypt: %w", err)
	}
	defer func() {
		if outBlob.Data != nil {
			_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))
		}
	}()

	out := make([]byte, outBlob.Size)
	if outBlob.Size > 0 {
		copy(out, unsafe.Slice(outBlob.Data, outBlob.Size))
	}
	return out, nil
}

func (c *DPAPICipher) Decrypt(blob []byte) (string, error) {
	if !c.available {
		return "", ErrCipherUnavailable
	}
	if len(blob) == 0 {
		return "", fmt.Errorf("dpapi decrypt: empty ciphertext")
	}
	inBlob := windows.DataBlob{
		Size: uint32(len(blob)),
		Data: &blob[0],
	}
	var outBlob windows.DataBlob
	err := windows.CryptUnprotectData(
		&inBlob,
		nil,
		nil,
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&outBlob,
	)
	if err != nil {
		return "", fmt.Errorf("dpapi decrypt: %w", err)
	}
	defer func() {
		if outBlob.Data != nil {
			_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))
		}
	}()

	plain := make([]byte, outBlob.Size)
	if outBlob.Size > 0 {
		copy(plain, unsafe.Slice(outBlob.Data, outBlob.Size))
	}
	return string(plain), nil
}
