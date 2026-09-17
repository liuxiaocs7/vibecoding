package config

import "testing"

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8090", true},
		{"localhost:8090", true},
		{"[::1]:8090", true},
		{"::1", true},
		{"0.0.0.0:8090", false},
		{"192.168.1.5:8090", false},
		{"", true},
	}
	for _, tc := range cases {
		if got := IsLoopbackAddr(tc.addr); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.addr, got, tc.want)
		}
	}
}

func TestValidateListenAuth(t *testing.T) {
	ok := &Config{Addr: "127.0.0.1:8090", Token: ""}
	if err := ok.ValidateListenAuth(); err != nil {
		t.Fatal(err)
	}
	need := &Config{Addr: "0.0.0.0:8090", Token: ""}
	if err := need.ValidateListenAuth(); err == nil {
		t.Fatal("expected error without token")
	}
	with := &Config{Addr: "0.0.0.0:8090", Token: "secret"}
	if err := with.ValidateListenAuth(); err != nil {
		t.Fatal(err)
	}
}
