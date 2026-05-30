package config

import "testing"

func TestParseAllowlistExactVsPrefix(t *testing.T) {
	rules := ParseAllowlist([]string{"-O2", "-std=*"})
	cases := []struct {
		flag string
		want bool
	}{
		{"-O2", true},
		{"-O3", false},
		{"-std=c++17", true},
		{"-std=", false},
		{"-std", false},
		{"", false},
	}

	for _, c := range cases {
		if got := Allows(rules, c.flag); got != c.want {
			t.Errorf("Allows(%q) = %v; want %v", c.flag, got, c.want)
		}
	}
}
