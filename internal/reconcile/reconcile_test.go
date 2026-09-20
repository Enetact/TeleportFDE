package reconcile

import (
	"context"
	"errors"
	"example.com/replica-control/internal/model"
	"testing"
)

type fakeStore struct {
	intent                                 model.Intent
	dep                                    model.Deployment
	hpa                                    bool
	intentErr, targetErr, scaleErr, hpaErr error
	changed                                bool
	getCalls, scaleCalls, statusCalls      int
	scaled                                 int32
	status                                 model.ReconcileStatus
}

func (f *fakeStore) GetIntent(context.Context, model.Target) (model.Intent, error) {
	f.getCalls++
	i := f.intent
	if f.changed && f.getCalls > 1 {
		i.Generation++
	}
	return i, f.intentErr
}
func (f *fakeStore) GetTarget(context.Context, model.Target) (model.Deployment, error) {
	return f.dep, f.targetErr
}
func (f *fakeStore) HasHPA(context.Context, model.Target) (bool, error) { return f.hpa, f.hpaErr }
func (f *fakeStore) UpdateScale(_ context.Context, _ model.Deployment, n int32) error {
	f.scaleCalls++
	f.scaled = n
	return f.scaleErr
}
func (f *fakeStore) WriteStatus(_ context.Context, _ model.Intent, s model.ReconcileStatus) error {
	f.statusCalls++
	f.status = s
	return nil
}
func newStore() *fakeStore {
	return &fakeStore{intent: model.Intent{Target: model.Target{Namespace: "default", Name: "demo"}, UID: "intent-uid", DeploymentUID: "dep-uid", Replicas: 3, Generation: 2}, dep: model.Deployment{Target: model.Target{Namespace: "default", Name: "demo"}, UID: "dep-uid", Replicas: 1, ReadyReplicas: 1, Generation: 1, ObservedGeneration: 1}}
}
func TestReconcile(t *testing.T) {
	for _, tc := range []struct {
		name          string
		change        func(*fakeStore)
		scale         int
		phase, reason string
		wantErr       bool
	}{
		{"drift", func(f *fakeStore) {}, 1, "Reconciling", "ScaleUpdated", false},
		{"ready", func(f *fakeStore) { f.dep.Replicas = 3; f.dep.ReadyReplicas = 3 }, 0, "Ready", "DesiredStateObserved", false},
		{"waiting pods", func(f *fakeStore) { f.dep.Replicas = 3 }, 0, "Reconciling", "WaitingForDeployment", false},
		{"old deployment generation", func(f *fakeStore) { f.dep.Replicas = 3; f.dep.ReadyReplicas = 3; f.dep.Generation = 2 }, 0, "Reconciling", "WaitingForDeployment", false},
		{"zero", func(f *fakeStore) { f.intent.Replicas = 0 }, 1, "Reconciling", "ScaleUpdated", false},
		{"zero ready", func(f *fakeStore) { f.intent.Replicas = 0; f.dep.Replicas = 0; f.dep.ReadyReplicas = 0 }, 0, "Ready", "DesiredStateObserved", false},
		{"HPA", func(f *fakeStore) { f.hpa = true }, 0, "Blocked", "HPAConflict", false},
		{"protected", func(f *fakeStore) { f.dep.Protected = true }, 0, "Blocked", "ProtectedDeployment", false},
		{"recreated deployment", func(f *fakeStore) { f.dep.UID = "replacement" }, 0, "Blocked", "DeploymentUIDMismatch", false},
		{"deleting deployment", func(f *fakeStore) { f.dep.Deleting = true }, 0, "Blocked", "DeploymentDeleting", false},
		{"missing deployment", func(f *fakeStore) { f.targetErr = model.E(model.NotFound, "missing", nil) }, 0, "Blocked", "DeploymentNotFound", false},
		{"missing intent", func(f *fakeStore) { f.intentErr = model.E(model.NotFound, "missing", nil) }, 0, "", "", false},
		{"deleting intent", func(f *fakeStore) { f.intent.Deleting = true }, 0, "", "", false},
		{"changed intent", func(f *fakeStore) { f.changed = true }, 0, "", "", true},
		{"scale conflict", func(f *fakeStore) { f.scaleErr = model.E(model.Conflict, "stale", nil) }, 1, "", "", true},
		{"HPA lookup denied", func(f *fakeStore) { f.hpaErr = errors.New("denied") }, 0, "", "", true},
		{"dependency unavailable", func(f *fakeStore) { f.targetErr = errors.New("offline") }, 0, "", "", true},
		{"invalid intent", func(f *fakeStore) { f.intent.Replicas = -1 }, 0, "Blocked", "InvalidReplicas", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStore()
			tc.change(f)
			err := (&Engine{Store: f}).Reconcile(context.Background(), f.intent.Target)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
			if f.scaleCalls != tc.scale || f.status.Phase != tc.phase || f.status.Reason != tc.reason {
				t.Fatalf("scale=%d status=%+v", f.scaleCalls, f.status)
			}
			if tc.name == "zero" && f.scaled != 0 {
				t.Fatal("zero not applied")
			}
			if f.statusCalls > 0 && f.status.ObservedGeneration != 2 {
				t.Fatal("wrong observed generation")
			}
		})
	}
}
