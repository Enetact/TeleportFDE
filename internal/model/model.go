// Package model defines transport-independent contracts and validation.
package model

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MaxReplicas caps replica requests in this development exercise.
const MaxReplicas int32 = 1000

// Code identifies an application error independently of HTTP or gRPC status codes.
type Code string

const (
	// InvalidArgument denotes malformed or out-of-range input.
	InvalidArgument Code = "invalid_argument"
	// NotFound denotes a missing target or Kubernetes resource.
	NotFound Code = "not_found"
	// Conflict denotes a stale version or a concurrently replaced resource.
	Conflict Code = "conflict"
	// Forbidden denotes an authorization or protected-target rejection.
	Forbidden Code = "forbidden"
	// Unavailable denotes a dependency or cancellation failure.
	Unavailable Code = "unavailable"
	// FailedPrecondition denotes a target state that prevents the operation.
	FailedPrecondition Code = "failed_precondition"
	// Internal denotes an unexpected application failure.
	Internal Code = "internal"
)

// Error carries a transport-neutral code, a message and an optional cause.
// Message is exposed by PublicError; keep backend details in Cause.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

// Error returns the message supplied by the caller.
func (e *Error) Error() string { return e.Message }

// Unwrap exposes the cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// E constructs an Error with the supplied code, message and optional cause.
func E(code Code, message string, cause error) error { return &Error{code, message, cause} }

// ErrorCode returns a wrapped Error's code, or classifies a generic error.
// Call it for a failed operation; nil is not a success sentinel in this helper.
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

// PublicError returns a wrapped Error's message or a generic failure description.
// It does not sanitize messages explicitly supplied through E.
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

// Target names one Deployment within a Kubernetes namespace.
type Target struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// Key returns namespace/name without validation; validate untrusted targets first.
func (t Target) Key() string { return t.Namespace + "/" + t.Name }

var dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidateNamespace checks a DNS label, optionally allowing an all-namespaces query.
func ValidateNamespace(s string, allowEmpty bool) error {
	if s == "" && allowEmpty {
		return nil
	}
	if len(s) == 0 || len(s) > 63 || !dnsLabel.MatchString(s) {
		return E(InvalidArgument, "namespace must be a DNS label of 1–63 characters", nil)
	}
	return nil
}

// Validate checks the namespace and Deployment name before a backend operation.
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

// ParseKey splits and validates a namespace/name work-queue key.
func ParseKey(key string) (Target, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return Target{}, E(InvalidArgument, "expected namespace/name", nil)
	}
	t := Target{parts[0], parts[1]}
	return t, t.Validate()
}

// ValidateReplicas accepts zero through MaxReplicas, inclusive.
func ValidateReplicas(n int32) error {
	if n < 0 || n > MaxReplicas {
		return E(InvalidArgument, fmt.Sprintf("replicas must be between 0 and %d", MaxReplicas), nil)
	}
	return nil
}

// Deployment is a copied view of a Deployment and optional level-5 intent.
// Replicas is the Deployment spec count; ReadyReplicas and AvailableReplicas
// come from status. DesiredReplicas is nil when no matching intent is present.
// ResourceVersion and IntentVersion belong to different Kubernetes objects.
// Protected and Deleting are internal write guards, omitted from API JSON.
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

// SetRequest carries a desired count and an optional concurrency precondition.
// A nil Replicas pointer means missing input; a pointer to zero requests scale-to-zero.
type SetRequest struct {
	Replicas *int32 `json:"replicas"`
	// At L2–4 this is the Deployment resourceVersion. At L5 it is the intent's
	// resourceVersion. Empty means an unconditional, last-successful-write wins.
	ExpectedVersion string `json:"expectedVersion,omitempty"`
}

// SetResult describes a direct scale write or a persisted level-5 intent.
// Version belongs to the object written. Neither result promises ready Pods.
type SetResult struct {
	Target
	Replicas int32  `json:"replicas"`
	Version  string `json:"version"`
	// Accepted means durable intent at L5, not that the Pods are already ready.
	Accepted bool `json:"accepted"`
}

// Backend supplies storage operations to the API service.
// Implementations receive concurrent calls and must honor request contexts.
// List accepts an empty namespace for all namespaces. Set interprets its version
// argument according to the configured level and returns conflicts to the caller.
type Backend interface {
	Get(context.Context, Target) (Deployment, error)
	List(context.Context, string) ([]Deployment, error)
	Set(context.Context, Target, int32, string) (SetResult, error)
}

// Intent is the desired count for one specific Deployment instance.
// UID identifies the intent; DeploymentUID prevents reuse for a same-name replacement.
// Generation and ResourceVersion support reconciliation and optimistic concurrency.
type Intent struct {
	Target
	UID             string
	DeploymentUID   string
	Replicas        int32
	Generation      int64
	ResourceVersion string
	Deleting        bool
}

// ReconcileStatus records the latest controller observation of an intent.
// ObservedGeneration identifies the intent spec evaluated by that pass.
// Phase and Reason describe progress; counts reflect observations, not promises.
type ReconcileStatus struct {
	ObservedGeneration int64  `json:"observedGeneration"`
	ObservedReplicas   int32  `json:"observedReplicas"`
	ReadyReplicas      int32  `json:"readyReplicas"`
	Phase              string `json:"phase"`
	Reason             string `json:"reason"`
}
