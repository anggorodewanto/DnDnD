package testutil

import "testing"

func TestProfBonusForLevel(t *testing.T) {
	cases := []struct {
		level int
		want  int32
	}{
		{0, 2},
		{-1, 2},
		{1, 2},
		{4, 2},
		{5, 3},
		{8, 3},
		{9, 4},
		{12, 4},
		{13, 5},
		{16, 5},
		{17, 6},
		{20, 6},
	}
	for _, c := range cases {
		if got := profBonusForLevel(c.level); got != c.want {
			t.Errorf("profBonusForLevel(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}
