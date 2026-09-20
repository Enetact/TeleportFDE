package kube

import (
	"context"
	"testing"
	"time"

	"example.com/replica-control/internal/model"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func sampleDeployment() *appsv1.Deployment {
	n := int32(2)
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo", UID: types.UID("deployment-uid"), ResourceVersion: "10"}, Spec: appsv1.DeploymentSpec{Replicas: &n}}
}
// TestCachedReadsDoNotCallAPIServer separates watch setup from API-read assertions.
// Clearing recorded actions avoids counting initial synchronization as a cache miss.
func TestCachedReadsDoNotCallAPIServer(t *testing.T) {
	d := sampleDeployment()
	k := fake.NewSimpleClientset(d)
	b := New(k, nil, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b.Start(ctx)
	if !cache.WaitForCacheSync(ctx.Done(), b.Deployments.Informer().HasSynced) {
		t.Fatal("cache did not sync")
	}
	k.ClearActions()
	for i := 0; i < 20; i++ {
		got, err := b.Get(ctx, model.Target{Namespace: "default", Name: "demo"})
		if err != nil || got.Replicas != 2 {
			t.Fatalf("got=%+v err=%v", got, err)
		}
		list, err := b.List(ctx, "")
		if err != nil || len(list) != 1 {
			t.Fatalf("list=%+v err=%v", list, err)
		}
	}
	for _, a := range k.Actions() {
		if a.GetVerb() == "get" || a.GetVerb() == "list" {
			t.Fatalf("cached read made Kubernetes action: %v", a)
		}
	}
	d = d.DeepCopy()
	n := int32(7)
	d.Spec.Replicas = &n
	if _, err := k.AppsV1().Deployments("default").Update(ctx, d, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	for {
		got, err := b.Get(ctx, model.Target{Namespace: "default", Name: "demo"})
		if err == nil && got.Replicas == 7 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("watch update not observed")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func TestDirectSetVersionAndZero(t *testing.T) {
	k := fake.NewSimpleClientset(sampleDeployment())
	b := New(k, nil, 2)
	k.PrependReactor("get", "deployments", func(a ktesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "scale" {
			return false, nil, nil
		}
		return true, &autoscalingv1.Scale{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "deployment-uid", ResourceVersion: "10"}, Spec: autoscalingv1.ScaleSpec{Replicas: 2}}, nil
	})
	k.PrependReactor("update", "deployments", func(a ktesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "scale" {
			return false, nil, nil
		}
		s := a.(ktesting.UpdateAction).GetObject().(*autoscalingv1.Scale).DeepCopy()
		s.ResourceVersion = "11"
		return true, s, nil
	})
	got, err := b.Set(context.Background(), model.Target{Namespace: "default", Name: "demo"}, 0, "10")
	if err != nil || got.Replicas != 0 || got.Version != "11" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	_, err = b.Set(context.Background(), model.Target{Namespace: "default", Name: "demo"}, 3, "stale")
	if model.ErrorCode(err) != model.Conflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}
func TestHPAConflict(t *testing.T) {
	h := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}, Spec: autoscalingv2.HorizontalPodAutoscalerSpec{ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", APIVersion: "apps/v1", Name: "demo"}}}
	k := fake.NewSimpleClientset(sampleDeployment(), h)
	b := New(k, nil, 2)
	_, err := b.Set(context.Background(), model.Target{Namespace: "default", Name: "demo"}, 3, "")
	if model.ErrorCode(err) != model.FailedPrecondition {
		t.Fatalf("HPA conflict not rejected: %v", err)
	}
}
// TestIntentIsDurableAndNotDirectScale checks intent creation in the fake API store.
// It does not prove persistence through an actual API-server or controller restart.
func TestIntentIsDurableAndNotDirectScale(t *testing.T) {
	k := fake.NewSimpleClientset(sampleDeployment())
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{IntentGVR: "ReplicaIntentList"})
	b := New(k, dyn, 5)
	result, err := b.Set(context.Background(), model.Target{Namespace: "default", Name: "demo"}, 4, "")
	if err != nil || !result.Accepted {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	u, err := dyn.Resource(IntentGVR).Namespace("default").Get(context.Background(), "demo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
	if n != 4 {
		t.Fatal(n)
	}
	uid, _, _ := unstructured.NestedString(u.Object, "spec", "deploymentUID")
	if uid != "deployment-uid" {
		t.Fatal(uid)
	}
	if len(u.GetOwnerReferences()) != 1 || u.GetOwnerReferences()[0].UID != "deployment-uid" {
		t.Fatal("owner UID missing")
	}
	for _, a := range k.Actions() {
		if a.GetVerb() == "update" && a.GetSubresource() == "scale" {
			t.Fatal("L5 API scaled directly instead of persisting intent")
		}
	}
}
func TestIntentUIDMismatchRejected(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": Group + "/v1alpha1", "kind": "ReplicaIntent", "metadata": map[string]any{"name": "demo", "namespace": "default", "resourceVersion": "10"}, "spec": map[string]any{"deploymentName": "demo", "deploymentUID": "old-deployment-uid", "replicas": int64(2)}}}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{IntentGVR: "ReplicaIntentList"}, u)
	b := New(fake.NewSimpleClientset(sampleDeployment()), dyn, 5)
	_, err := b.Set(context.Background(), model.Target{Namespace: "default", Name: "demo"}, 4, "")
	if model.ErrorCode(err) != model.FailedPrecondition {
		t.Fatalf("replacement not protected: %v", err)
	}
}
