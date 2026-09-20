// Package kube adapts the core contracts to client-go. Informer objects are
// immutable: handlers convert them to value objects and never mutate the cache.
package kube

import (
	"context"
	"errors"
	"sort"
	"strings"

	"example.com/replica-control/internal/model"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	autoscalinginformers "k8s.io/client-go/informers/autoscaling/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const Group = "replicas.reference.example.com"

var IntentGVR = schema.GroupVersionResource{Group: Group, Version: "v1alpha1", Resource: "replicaintents"}

type Backend struct {
	Kube           kubernetes.Interface
	Dynamic        dynamic.Interface
	Level          int
	Factory        informers.SharedInformerFactory
	Deployments    appsinformers.DeploymentInformer
	HPAs           autoscalinginformers.HorizontalPodAutoscalerInformer
	DynamicFactory dynamicinformer.DynamicSharedInformerFactory
	Intents        informers.GenericInformer
}

func New(k kubernetes.Interface, d dynamic.Interface, level int) *Backend {
	b := &Backend{Kube: k, Dynamic: d, Level: level}
	b.Factory = informers.NewSharedInformerFactory(k, 0)
	b.Deployments = b.Factory.Apps().V1().Deployments()
	if level >= 4 {
		b.Deployments.Informer()
	}
	if level == 5 {
		b.HPAs = b.Factory.Autoscaling().V2().HorizontalPodAutoscalers()
		b.HPAs.Informer()
		b.DynamicFactory = dynamicinformer.NewDynamicSharedInformerFactory(d, 0)
		b.Intents = b.DynamicFactory.ForResource(IntentGVR)
		b.Intents.Informer()
	}
	return b
}
func (b *Backend) Start(ctx context.Context) {
	if b.Level >= 4 {
		b.Factory.Start(ctx.Done())
	}
	if b.Level == 5 {
		b.DynamicFactory.Start(ctx.Done())
	}
}
func (b *Backend) Synced() bool {
	if b.Level < 4 {
		return true
	}
	if !b.Deployments.Informer().HasSynced() {
		return false
	}
	return b.Level != 5 || (b.Intents.Informer().HasSynced() && b.HPAs.Informer().HasSynced())
}

// Health does live, bounded checks; it is not the read API's data path.
func (b *Backend) Health(ctx context.Context) error {
	if _, err := b.Kube.AppsV1().Deployments("").List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		return err
	}
	if b.Level == 5 {
		_, err := b.Dynamic.Resource(IntentGVR).List(ctx, metav1.ListOptions{Limit: 1})
		return err
	}
	return nil
}
func (b *Backend) Get(ctx context.Context, t model.Target) (model.Deployment, error) {
	var d *appsv1.Deployment
	var err error
	if b.Level >= 4 {
		if !b.Synced() {
			return model.Deployment{}, model.E(model.Unavailable, "deployment cache is not synchronized", nil)
		}
		d, err = b.Deployments.Lister().Deployments(t.Namespace).Get(t.Name)
	} else {
		d, err = b.Kube.AppsV1().Deployments(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
	}
	if err != nil {
		return model.Deployment{}, mapError(err)
	}
	out := deployment(d)
	if b.Level == 5 {
		if err := b.decorateIntent(&out); err != nil {
			return model.Deployment{}, err
		}
	}
	return out, nil
}
func (b *Backend) List(ctx context.Context, namespace string) ([]model.Deployment, error) {
	out := []model.Deployment{}
	if b.Level >= 4 {
		if !b.Synced() {
			return nil, model.E(model.Unavailable, "deployment cache is not synchronized", nil)
		}
		var ds []*appsv1.Deployment
		var err error
		if namespace == "" {
			ds, err = b.Deployments.Lister().List(labels.Everything())
		} else {
			ds, err = b.Deployments.Lister().Deployments(namespace).List(labels.Everything())
		}
		if err != nil {
			return nil, mapError(err)
		}
		for _, d := range ds {
			out = append(out, deployment(d))
		}
	} else {
		// Explicit pagination avoids silently dropping items from a large list.
		opts := metav1.ListOptions{Limit: 500}
		for {
			list, err := b.Kube.AppsV1().Deployments(namespace).List(ctx, opts)
			if err != nil {
				return nil, mapError(err)
			}
			for i := range list.Items {
				out = append(out, deployment(&list.Items[i]))
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	if b.Level == 5 {
		for i := range out {
			if err := b.decorateIntent(&out[i]); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target.Key() < out[j].Target.Key() })
	return out, nil
}
func (b *Backend) decorateIntent(out *model.Deployment) error {
	obj, err := b.Intents.Lister().ByNamespace(out.Namespace).Get(out.Name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return mapError(err)
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return model.E(model.Internal, "invalid cached intent type", nil)
	}
	intent, err := parseIntent(u)
	if err != nil {
		return err
	}
	// An old owner's intent must not be presented as the recreated Deployment's target.
	if intent.DeploymentUID != out.UID {
		return nil
	}
	desired := intent.Replicas
	out.DesiredReplicas = &desired
	out.IntentVersion = intent.ResourceVersion
	out.ReconcilePhase, _, _ = unstructured.NestedString(u.Object, "status", "phase")
	return nil
}
func (b *Backend) Set(ctx context.Context, t model.Target, n int32, expected string) (model.SetResult, error) {
	if b.Level < 2 {
		return model.SetResult{}, model.E(model.Forbidden, "this level is read only", nil)
	}
	dep, err := b.GetTarget(ctx, t)
	if err != nil {
		return model.SetResult{}, err
	}
	if err := b.checkWritable(ctx, dep); err != nil {
		return model.SetResult{}, err
	}
	if b.Level == 5 {
		return b.setIntent(ctx, dep, n, expected)
	}
	var out model.SetResult
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		scale, err := b.Kube.AppsV1().Deployments(t.Namespace).GetScale(ctx, t.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if string(scale.UID) != dep.UID {
			return model.E(model.Conflict, "deployment was replaced; read it again", nil)
		}
		if expected != "" && expected != scale.ResourceVersion {
			return model.E(model.Conflict, "deployment version has changed", nil)
		}
		scale = scale.DeepCopy()
		scale.Spec.Replicas = n
		updated, err := b.Kube.AppsV1().Deployments(t.Namespace).UpdateScale(ctx, t.Name, scale, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		out = model.SetResult{Target: t, Replicas: updated.Spec.Replicas, Version: updated.ResourceVersion, Accepted: false}
		return nil
	})
	return out, mapError(err)
}
func (b *Backend) GetTarget(ctx context.Context, t model.Target) (model.Deployment, error) {
	d, err := b.Kube.AppsV1().Deployments(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
	if err != nil {
		return model.Deployment{}, mapError(err)
	}
	return deployment(d), nil
}
func (b *Backend) HasHPA(ctx context.Context, t model.Target) (bool, error) {
	hpas, err := b.Kube.AutoscalingV2().HorizontalPodAutoscalers(t.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, mapError(err)
	}
	for _, h := range hpas.Items {
		ref := h.Spec.ScaleTargetRef
		if ref.Kind == "Deployment" && ref.Name == t.Name && strings.HasPrefix(ref.APIVersion, "apps/") {
			return true, nil
		}
	}
	return false, nil
}
func (b *Backend) checkWritable(ctx context.Context, d model.Deployment) error {
	if d.Deleting {
		return model.E(model.FailedPrecondition, "deployment is being deleted", nil)
	}
	if d.Protected {
		return model.E(model.Forbidden, "deployment is protected from scaling", nil)
	}
	hpa, err := b.HasHPA(ctx, d.Target)
	if err != nil {
		return err
	}
	if hpa {
		return model.E(model.FailedPrecondition, "an HPA already controls this deployment", nil)
	}
	return nil
}
func deployment(d *appsv1.Deployment) model.Deployment {
	n := int32(1)
	if d.Spec.Replicas != nil {
		n = *d.Spec.Replicas
	}
	protected := d.Annotations[Group+"/protected"] == "true"
	switch d.Namespace {
	case "kube-system", "kube-public", "kube-node-lease":
		protected = true
	}
	return model.Deployment{Target: model.Target{Namespace: d.Namespace, Name: d.Name}, UID: string(d.UID),
		ResourceVersion: d.ResourceVersion, Replicas: n, ReadyReplicas: d.Status.ReadyReplicas,
		AvailableReplicas: d.Status.AvailableReplicas, Generation: d.Generation, ObservedGeneration: d.Status.ObservedGeneration,
		Protected: protected, Deleting: d.DeletionTimestamp != nil}
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var own *model.Error
	if errors.As(err, &own) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	switch {
	case apierrors.IsNotFound(err):
		return model.E(model.NotFound, "Kubernetes resource was not found", err)
	case apierrors.IsAlreadyExists(err), apierrors.IsConflict(err):
		return model.E(model.Conflict, "resource changed; read the current version and retry", err)
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		return model.E(model.Forbidden, "Kubernetes access denied", err)
	case apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		return model.E(model.InvalidArgument, "Kubernetes rejected the resource", err)
	default:
		return model.E(model.Unavailable, "Kubernetes operation failed", err)
	}
}
