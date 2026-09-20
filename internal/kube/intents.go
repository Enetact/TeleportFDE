package kube

import (
	"context"
	"encoding/json"
	"reflect"

	"example.com/replica-control/internal/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func parseIntent(u *unstructured.Unstructured) (model.Intent, error) {
	name, ok, err := unstructured.NestedString(u.Object, "spec", "deploymentName")
	if err != nil || !ok || name != u.GetName() {
		return model.Intent{}, model.E(model.FailedPrecondition, "intent name must match deploymentName", err)
	}
	uid, ok, err := unstructured.NestedString(u.Object, "spec", "deploymentUID")
	if err != nil || !ok || uid == "" {
		return model.Intent{}, model.E(model.FailedPrecondition, "intent has no deployment UID", err)
	}
	n, ok, err := unstructured.NestedInt64(u.Object, "spec", "replicas")
	if err != nil || !ok || n < 0 || n > int64(model.MaxReplicas) {
		return model.Intent{}, model.E(model.FailedPrecondition, "invalid replica count in intent", err)
	}
	return model.Intent{Target: model.Target{Namespace: u.GetNamespace(), Name: name}, UID: string(u.GetUID()), DeploymentUID: uid,
		Replicas: int32(n), Generation: u.GetGeneration(), ResourceVersion: u.GetResourceVersion(), Deleting: u.GetDeletionTimestamp() != nil}, nil
}
func (b *Backend) GetIntent(ctx context.Context, t model.Target) (model.Intent, error) {
	u, err := b.Dynamic.Resource(IntentGVR).Namespace(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
	if err != nil {
		return model.Intent{}, mapError(err)
	}
	return parseIntent(u)
}
func (b *Backend) setIntent(ctx context.Context, dep model.Deployment, n int32, expected string) (model.SetResult, error) {
	var out model.SetResult
	client := b.Dynamic.Resource(IntentGVR).Namespace(dep.Namespace)
	err := retry.OnError(retry.DefaultBackoff, func(err error) bool { return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) }, func() error {
		u, err := client.Get(ctx, dep.Name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			if expected != "" {
				return model.E(model.Conflict, "intent no longer exists", nil)
			}
			u = &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": Group + "/v1alpha1", "kind": "ReplicaIntent",
				"metadata": map[string]any{"name": dep.Name, "namespace": dep.Namespace,
					"labels": map[string]any{"app.kubernetes.io/managed-by": "replica-control"}},
				"spec": map[string]any{"deploymentName": dep.Name, "deploymentUID": dep.UID, "replicas": int64(n)},
			}}
			// No blockOwnerDeletion permission is needed. Deleting the Deployment
			// lets Kubernetes garbage-collect its per-instance intent.
			u.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: dep.Name, UID: types.UID(dep.UID)}})
			u, err = client.Create(ctx, u, metav1.CreateOptions{})
		case err != nil:
			return err
		default:
			previous, parseErr := parseIntent(u)
			if parseErr != nil {
				return parseErr
			}
			if previous.Deleting {
				return model.E(model.FailedPrecondition, "intent is being deleted", nil)
			}
			if previous.DeploymentUID != dep.UID {
				return model.E(model.FailedPrecondition, "intent belongs to a different deployment UID; remove the stale intent", nil)
			}
			if expected != "" && expected != u.GetResourceVersion() {
				return model.E(model.Conflict, "intent version has changed", nil)
			}
			if previous.Replicas != n {
				u = u.DeepCopy()
				if err := unstructured.SetNestedField(u.Object, int64(n), "spec", "replicas"); err != nil {
					return err
				}
				u, err = client.Update(ctx, u, metav1.UpdateOptions{})
			}
		}
		if err != nil {
			return err
		}
		out = model.SetResult{Target: dep.Target, Replicas: n, Version: u.GetResourceVersion(), Accepted: true}
		return nil
	})
	return out, mapError(err)
}

// UpdateScale checks UID and resourceVersion again to avoid scaling a same-name
// replacement or silently overwriting an intervening update.
func (b *Backend) UpdateScale(ctx context.Context, dep model.Deployment, n int32) error {
	scale, err := b.Kube.AppsV1().Deployments(dep.Namespace).GetScale(ctx, dep.Name, metav1.GetOptions{})
	if err != nil {
		return mapError(err)
	}
	if string(scale.UID) != dep.UID || scale.ResourceVersion != dep.ResourceVersion {
		return model.E(model.Conflict, "deployment changed during reconciliation", nil)
	}
	scale = scale.DeepCopy()
	scale.Spec.Replicas = n
	_, err = b.Kube.AppsV1().Deployments(dep.Namespace).UpdateScale(ctx, dep.Name, scale, metav1.UpdateOptions{})
	return mapError(err)
}
func (b *Backend) WriteStatus(ctx context.Context, intent model.Intent, status model.ReconcileStatus) error {
	client := b.Dynamic.Resource(IntentGVR).Namespace(intent.Namespace)
	// Convert through JSON to keep the status contract independent of Kubernetes.
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	// Unstructured Kubernetes integers must be int64, not JSON float64.
	m["observedGeneration"] = status.ObservedGeneration
	m["observedReplicas"] = int64(status.ObservedReplicas)
	m["readyReplicas"] = int64(status.ReadyReplicas)
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		u, err := client.Get(ctx, intent.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if string(u.GetUID()) != intent.UID || u.GetGeneration() != intent.Generation || u.GetDeletionTimestamp() != nil {
			return model.E(model.Conflict, "intent changed before status could be written", nil)
		}
		current, _, _ := unstructured.NestedMap(u.Object, "status")
		if reflect.DeepEqual(current, m) {
			return nil
		}
		u = u.DeepCopy()
		u.Object["status"] = m
		_, err = client.UpdateStatus(ctx, u, metav1.UpdateOptions{})
		return err
	})
	return mapError(err)
}
