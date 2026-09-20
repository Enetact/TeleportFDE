// Package model defines transport-independent contracts and validation.
package model

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const MaxReplicas int32 = 1000

type Code string

const (
	InvalidArgument    Code = "invalid_argument"
	NotFound           Code = "not_found"
	Conflict           Code = "conflict"
	Forbidden          Code = "forbidden"
	Unavailable        Code = "unavailable"
	FailedPrecondition Code = "failed_precondition"
	Internal           Code = "internal"
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string                       { return e.Message }
func (e *Error) Unwrap() error                       { return e.Cause }
func E(code Code, message string, cause error) error { return &Error{code, message, cause} }
func ErrorCode(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return Unavailable
	}
	return Internal
}
func PublicError(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Message
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "operation timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "operation canceled"
	}
	return "internal server error"
}

type Target struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (t Target) Key() string { return t.Namespace + "/" + t.Name }

var dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func ValidateNamespace(s string, allowEmpty bool) error {
	if s == "" && allowEmpty {
		return nil
	}
	if len(s) == 0 || len(s) > 63 || !dnsLabel.MatchString(s) {
		return E(InvalidArgument, "namespace must be a DNS label of 1–63 characters", nil)
	}
	return nil
}
func (t Target) Validate() error {
	if err := ValidateNamespace(t.Namespace, false); err != nil {
		return err
	}
	if len(t.Name) == 0 || len(t.Name) > 253 {
		return E(InvalidArgument, "deployment name must contain 1–253 characters", nil)
	}
	for _, label := range strings.Split(t.Name, ".") {
		if len(label) > 63 || !dnsLabel.MatchString(label) {
			return E(InvalidArgument, "deployment name must be a DNS subdomain", nil)
		}
	}
	return nil
}
func ParseKey(key string) (Target, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return Target{}, E(InvalidArgument, "expected namespace/name", nil)
	}
	t := Target{parts[0], parts[1]}
	return t, t.Validate()
}
func ValidateReplicas(n int32) error {
	if n < 0 || n > MaxReplicas {
		return E(InvalidArgument, fmt.Sprintf("replicas must be between 0 and %d", MaxReplicas), nil)
	}
	return nil
}

type Deployment struct {
	Target
	UID                string `json:"uid"`
	ResourceVersion    string `json:"resourceVersion"`
	Replicas           int32  `json:"replicas"`
	ReadyReplicas      int32  `json:"readyReplicas"`
	AvailableReplicas  int32  `json:"availableReplicas"`
	Generation         int64  `json:"generation"`
	ObservedGeneration int64  `json:"observedGeneration"`
	DesiredReplicas    *int32 `json:"desiredReplicas,omitempty"`
	IntentVersion      string `json:"intentVersion,omitempty"`
	ReconcilePhase     string `json:"reconcilePhase,omitempty"`
	Protected          bool   `json:"-"`
	Deleting           bool   `json:"-"`
}

type SetRequest struct {
	Replicas *int32 `json:"replicas"`
	// At L2–4 this is the Deployment resourceVersion. At L5 it is the intent's
	// resourceVersion. Empty means an unconditional, last-successful-write wins.
	ExpectedVersion string `json:"expectedVersion,omitempty"`
}
type SetResult struct {
	Target
	Replicas int32  `json:"replicas"`
	Version  string `json:"version"`
	// Accepted means durable intent at L5, not that the Pods are already ready.
	Accepted bool `json:"accepted"`
}

type Backend interface {
	Get(context.Context, Target) (Deployment, error)
	List(context.Context, string) ([]Deployment, error)
	Set(context.Context, Target, int32, string) (SetResult, error)
}

type Intent struct {
	Target
	UID             string
	DeploymentUID   string
	Replicas        int32
	Generation      int64
	ResourceVersion string
	Deleting        bool
}

type ReconcileStatus struct {
	ObservedGeneration int64  `json:"observedGeneration"`
	ObservedReplicas   int32  `json:"observedReplicas"`
	ReadyReplicas      int32  `json:"readyReplicas"`
	Phase              string `json:"phase"`
	Reason             string `json:"reason"`
}
