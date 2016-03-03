package address

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func ip(s string) Address {
	addr, _ := ParseIP(s)
	return addr
}

func TestCIDRs(t *testing.T) {
	start := ip("192.168.1.42")
	end := ip("192.168.2.42")
	offset := Subtract(end, start)
	r := NewRange(start, offset)
	// for [192.168.1.42,192.168.2.42)
	expectedCIDRs := []CIDR{
		CIDR{ip("192.168.1.42"), 31},
		CIDR{ip("192.168.1.44"), 30},
		CIDR{ip("192.168.1.48"), 28},
		CIDR{ip("192.168.1.64"), 26},
		CIDR{ip("192.168.1.128"), 25},
		CIDR{ip("192.168.2.0"), 27},
		CIDR{ip("192.168.2.32"), 29},
		CIDR{ip("192.168.2.40"), 31},
	}
	cidrs := r.CIDRs()

	require.Equal(t, len(cidrs), len(expectedCIDRs), "")
	require.Equal(t, expectedCIDRs, cidrs, "")
}

func TestSingleCIDR(t *testing.T) {
	r := NewRange(ip("192.168.1.0"), 256)
	expectedCIDR := CIDR{ip("192.168.1.0"), 24}
	cidrs := r.CIDRs()

	require.Equal(t, len(cidrs), 1)
	require.Equal(t, expectedCIDR, cidrs[0])

	r = NewRange(ip("192.168.1.1"), 1)
	expectedCIDR = CIDR{ip("192.168.1.1"), 32}
	cidrs = r.CIDRs()

	require.Equal(t, len(cidrs), 1)
	require.Equal(t, expectedCIDR, cidrs[0])
}

func TestIsCIDR(t *testing.T) {
	require.True(t, NewRange(ip("10.20.0.0"), 256).IsCIDR(), "")
	require.True(t, NewRange(ip("10.20.0.1"), 1).IsCIDR(), "")
	require.False(t, NewRange(ip("10.20.0.1"), 2).IsCIDR(), "")
	require.False(t, NewRange(ip("10.20.0.0"), 254).IsCIDR(), "")
	require.True(t, NewRange(ip("10.0.0.0"), 4).IsCIDR(), "")
}