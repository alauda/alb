package controller

import (
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"testing"
)

func TestRule_GetPriority(t *testing.T) {
	type fields struct {
		Priority int
		DSL      string
		DSLX     v1.DSLX
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "include priority",
			fields: fields{
				Priority: 100,
				DSL:      "(START_WITH URL /)",
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 100,
		},
		{
			name: "no priority with dsl 1",
			fields: fields{
				DSL: "(START_WITH URL /)",
			},
			want: 10000 + 100 + len("(START_WITH URL /)"),
		},
		{
			name: "no priority with dsl 2",
			fields: fields{
				DSL: "(START_WITH URL /lorem)",
			},
			want: 10000 + 100 + len("(START_WITH URL /lorem)"),
		},
		{
			name: "no priority with dslx",
			fields: fields{
				DSL: "(START_WITH URL /)",
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100 + len("(START_WITH URL /)"),
		},
		{
			name: "no priority with dslx",
			fields: fields{
				DSL: "(START_WITH URL /)",
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100 + len("(START_WITH URL /)"),
		},
		{
			name: "no priority with complex dslx",
			fields: fields{
				DSL: "(AND (OR (START_WITH URL /k8s) (START_WITH URL /kubernetes)) (EQ COOKIE test lorem))",
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/k8s"}, {"START_WITH", "/kubernetes"}},
						Type:   "URL",
					},
					{
						Values: [][]string{{"EQ", "lorem"}},
						Type:   "COOKIE",
						Key:    "test",
					},
				},
			},
			want: 10000 + 100 + 100 + 10000 + 100 + len("(AND (OR (START_WITH URL /k8s) (START_WITH URL /kubernetes)) (EQ COOKIE test lorem))"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := Rule{
				Priority: tt.fields.Priority,
				DSL:      tt.fields.DSL,
				DSLX:     tt.fields.DSLX,
			}
			if got := rl.GetPriority(); got != tt.want {
				t.Errorf("GetPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}
