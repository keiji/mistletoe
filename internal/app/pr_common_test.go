package app

import (
	"reflect"
	"testing"
)

func TestSortPrs(t *testing.T) {
	tests := []struct {
		name string
		prs  []PrInfo
		want []int // Expected order of numbers
	}{
		{
			name: "Sort by State Priority",
			prs: []PrInfo{
				{Number: 1, State: "MERGED"},
				{Number: 2, State: "OPEN"},
				{Number: 3, State: "CLOSED"},
			},
			want: []int{2, 1, 3}, // Open(0) < Merged(2) < Closed(3)
		},
		{
			name: "Sort Open Draft vs Open",
			prs: []PrInfo{
				{Number: 1, State: "OPEN", IsDraft: true},
				{Number: 2, State: "OPEN", IsDraft: false},
			},
			want: []int{2, 1}, // Open(0) < Draft(1)
		},
		{
			name: "Sort by Number Descending within State",
			prs: []PrInfo{
				{Number: 10, State: "OPEN"},
				{Number: 20, State: "OPEN"},
			},
			want: []int{20, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortPrs(tt.prs)
			var got []int
			for _, p := range tt.prs {
				got = append(got, p.Number)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortPrs() got %v, want %v", got, tt.want)
			}
		})
	}
}
