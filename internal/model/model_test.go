package model

import (
	"context"
	"errors"
	"testing"
)

func TestTargetValidation(t *testing.T) {
	for _, tc := range []struct {
		ns, name string
		valid    bool
	}{
		{"default", "demo", true}, {"team-a", "api.v2", true}, {"", "demo", false}, {"UPPER", "demo", false}, {"default", "../secrets", false},
		{"default", "-demo", false}, {"default", "demo-", false}, {"default", "demo..x", false}, {"a_b", "demo", false}, {"a", "a", true},
	} {
		t.Run(tc.ns+"/"+tc.name, func(t *testing.T) {
			err := (Target{tc.ns, tc.name}).Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
		})
	}
}
func TestReplicas(t *testing.T) {
	for _, n := range []int32{-1, 0, 1, 1000, 1001} {
		valid := n >= 0 && n <= MaxReplicas
		if (ValidateReplicas(n) == nil) != valid {
			t.Errorf("unexpected validity for %d", n)
		}
	}
}
func TestErrors(t *testing.T) {
	cause := errors.New("sensitive backend details")
	err := E(Unavailable, "dependency unavailable", cause)
	if !errors.Is(err, cause) || ErrorCode(err) != Unavailable || PublicError(err) != "dependency unavailable" {
		t.Fatal("error contract failed")
	}
	if PublicError(cause) != "internal server error" {
		t.Fatal("raw backend error leaked")
	}
	if ErrorCode(context.DeadlineExceeded) != Unavailable {
		t.Fatal("deadline not mapped")
	}
}
func TestParseKey(t *testing.T) {
	if _, err := ParseKey("default/demo"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"demo", "a/b/c", "/x"} {
		if _, err := ParseKey(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
