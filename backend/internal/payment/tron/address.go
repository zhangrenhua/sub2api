package tron

import (
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

// IsValidAddress reports whether s is a well-formed TRON Base58Check address.
func IsValidAddress(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s[0] != 'T' {
		return false
	}
	addr, err := address.Base58ToAddress(s)
	if err != nil {
		return false
	}
	return addr.IsValid()
}
