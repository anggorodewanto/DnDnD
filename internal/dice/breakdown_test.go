package dice

import "testing"

func TestFormatValuedBreakdown(t *testing.T) {
	tests := []struct {
		name       string
		d20        D20Result
		bonusExpr  string
		bonusTotal int
		grandTotal int
		want       string
	}{
		{
			name:       "normal roll, positive modifier",
			d20:        D20Result{Rolls: []int{13}, Chosen: 13, Modifier: 2, Total: 15},
			grandTotal: 15,
			want:       "d20(13) + 2 = 15",
		},
		{
			name:       "with effect die folds into single total",
			d20:        D20Result{Rolls: []int{13}, Chosen: 13, Modifier: 2, Total: 15},
			bonusExpr:  "1d4",
			bonusTotal: 2,
			grandTotal: 17,
			want:       "d20(13) + 2 + 1d4(2) = 17",
		},
		{
			name:       "zero modifier is omitted",
			d20:        D20Result{Rolls: []int{10}, Chosen: 10, Modifier: 0, Total: 10},
			grandTotal: 10,
			want:       "d20(10) = 10",
		},
		{
			name:       "negative modifier renders as subtraction",
			d20:        D20Result{Rolls: []int{10}, Chosen: 10, Modifier: -2, Total: 8},
			grandTotal: 8,
			want:       "d20(10) - 2 = 8",
		},
		{
			name:       "advantage shows both faces and the kept one",
			d20:        D20Result{Rolls: []int{18, 4}, Chosen: 18, Modifier: 2, Total: 20, Mode: Advantage},
			grandTotal: 20,
			want:       "d20(18/4→18) + 2 = 20",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatValuedBreakdown(tc.d20, tc.bonusExpr, tc.bonusTotal, tc.grandTotal)
			if got != tc.want {
				t.Errorf("FormatValuedBreakdown() = %q, want %q", got, tc.want)
			}
		})
	}
}
