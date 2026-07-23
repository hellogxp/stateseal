package main

import "testing"

func TestUIAddressMustRemainLoopbackOnly(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "localhost:9137", "[::1]:0"} {
		if err := validateLoopbackAddress(address); err != nil {
			t.Errorf("loopback address %q rejected: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:9137", "192.168.1.10:9137", ":9137", "not-an-address"} {
		if err := validateLoopbackAddress(address); err == nil {
			t.Errorf("non-loopback address %q accepted", address)
		}
	}
}
