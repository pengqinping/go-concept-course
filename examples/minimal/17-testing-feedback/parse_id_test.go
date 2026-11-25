package main

import (
	"errors"
	"testing"
)

func TestParseID(t *testing.T) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseID(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseID(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
			if tc.wantErr == nil && got != tc.want {
				t.Fatalf("ParseID(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}
