package controller

import "testing"

func TestPolicies_Less(t *testing.T) {
	type args struct {
		i int
		j int
	}
	tests := []struct {
		name string
		p    Policies
		args args
		want bool
	}{
		{
			"1",
			[]*Policy{
				{Priority: 100, RawPriority: 5},
				{Priority: 100, RawPriority: 5},
			},
			args{0, 1},
			false,
		},
		{
			"2",
			[]*Policy{
				{Priority: 100, RawPriority: 4},
				{Priority: 100, RawPriority: 5},
			},
			args{0, 1},
			true,
		},
		{
			"3",
			[]*Policy{
				{Priority: 99, RawPriority: 5},
				{Priority: 100, RawPriority: 5},
			},
			args{0, 1},
			false,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Less(tt.args.i, tt.args.j); got != tt.want {
				t.Errorf("Less() = %v, want %v", got, tt.want)
			}
		})
	}
}
