// Package reconcile contains the controller's dependency-free decision logic.
package reconcile

import (
	"context"
	"example.com/replica-control/internal/model"
)

// Store contains the operations one reconciliation pass needs.
// Implementations must honor contexts and reject stale writes; tests can use a fake
// without importing Kubernetes clients into the decision logic.
type Store interface {
	GetIntent(context.Context, model.Target) (model.Intent, error)
	GetTarget(context.Context, model.Target) (model.Deployment, error)
	HasHPA(context.Context, model.Target) (bool, error)
	UpdateScale(context.Context, model.Deployment, int32) error
	WriteStatus(context.Context, model.Intent, model.ReconcileStatus) error
}

// Engine reconciles desired counts through its configured Store.
// Keep Store fixed after construction. The controller serializes passes per target.
type Engine struct{ Store Store }

// Reconcile is idempotent. A conflict is retried by the work queue, never hidden.
// Changes to the intent and the Deployment cannot be one atomic transaction.
// Re-reading narrows that race; events and periodic requeues allow another pass.
func (e *Engine) Reconcile(ctx context.Context, t model.Target) error {
	intent, err := e.Store.GetIntent(ctx, t)
	if model.ErrorCode(err) == model.NotFound && err != nil {
		return nil
	}
	if err != nil {
		return err
	}
	if intent.Deleting {
		return nil
	}
	st := model.ReconcileStatus{ObservedGeneration: intent.Generation}
	if err := model.ValidateReplicas(intent.Replicas); err != nil {
		st.Phase = "Blocked"
		st.Reason = "InvalidReplicas"
		return e.Store.WriteStatus(ctx, intent, st)
	}
	dep, err := e.Store.GetTarget(ctx, t)
	if err != nil {
		if model.ErrorCode(err) != model.NotFound {
			return err
		}
		st.Phase = "Blocked"
		st.Reason = "DeploymentNotFound"
		return e.Store.WriteStatus(ctx, intent, st)
	}
	st.ObservedReplicas = dep.Replicas
	st.ReadyReplicas = dep.ReadyReplicas
	block := func(reason string) error {
		st.Phase = "Blocked"
		st.Reason = reason
		return e.Store.WriteStatus(ctx, intent, st)
	}
	if intent.DeploymentUID != dep.UID {
		return block("DeploymentUIDMismatch")
	}
	if dep.Deleting {
		return block("DeploymentDeleting")
	}
	if dep.Protected {
		return block("ProtectedDeployment")
	}
	hpa, err := e.Store.HasHPA(ctx, t)
	if err != nil {
		return err
	}
	if hpa {
		return block("HPAConflict")
	}
	if dep.Replicas != intent.Replicas {
		fresh, err := e.Store.GetIntent(ctx, t)
		if err != nil {
			return err
		}
		if fresh.UID != intent.UID || fresh.Generation != intent.Generation || fresh.Deleting {
			return model.E(model.Conflict, "intent changed during reconciliation", nil)
		}
		if err := e.Store.UpdateScale(ctx, dep, intent.Replicas); err != nil {
			return err
		}
		st.Phase = "Reconciling"
		st.Reason = "ScaleUpdated"
		// Keep the observed count from the actual read, not a fabricated ready count.
	} else if dep.ReadyReplicas == intent.Replicas && dep.ObservedGeneration >= dep.Generation {
		st.Phase = "Ready"
		st.Reason = "DesiredStateObserved"
	} else {
		st.Phase = "Reconciling"
		st.Reason = "WaitingForDeployment"
	}
	return e.Store.WriteStatus(ctx, intent, st)
}
