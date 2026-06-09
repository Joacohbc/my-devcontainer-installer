package domain

import "testing"

func TestBindLoopback(t *testing.T) {
	cases := []struct{ in, want string }{
		{"8080:80", "127.0.0.1:8080:80"},
		{"2222:22", "127.0.0.1:2222:22"},
		{"80", "127.0.0.1::80"},
		{"80/udp", "127.0.0.1::80/udp"},
		{"8080:80/tcp", "127.0.0.1:8080:80/tcp"},
		{"0.0.0.0:8080:80", "0.0.0.0:8080:80"},
		{"192.168.1.5:8080:80", "192.168.1.5:8080:80"},
		{"127.0.0.1::80", "127.0.0.1::80"},
	}
	for _, c := range cases {
		if got := BindLoopback(c.in); got != c.want {
			t.Errorf("BindLoopback(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
