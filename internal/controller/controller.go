// Package controller wires informer events and a bounded retry queue to the
// reconciliation engine. Only the elected leader calls Run; all Pods serve APIs.
package controller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"example.com/replica-control/internal/kube"
	"example.com/replica-control/internal/model"
	"example.com/replica-control/internal/reconcile"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	backend *kube.Backend
	engine  *reconcile.Engine
	queue   workqueue.TypedRateLimitingInterface[string]
	log     *slog.Logger
}

func New(b *kube.Backend, log *slog.Logger) (*Controller, error) {
	c := &Controller{backend: b, engine: &reconcile.Engine{Store: b}, log: log,
		queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())}
	enqueue := func(obj any) {
		key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
		if err == nil {
			c.queue.Add(key)
		}
	}
	if _, err := b.Intents.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: enqueue, DeleteFunc: enqueue,
		UpdateFunc: func(old, new any) {
			o, e1 := meta.Accessor(old)
			n, e2 := meta.Accessor(new)
			if e1 == nil && e2 == nil && (o.GetGeneration() != n.GetGeneration() || n.GetDeletionTimestamp() != nil) {
				enqueue(new)
			}
		},
	}); err != nil {
		return nil, err
	}
	// Deployment status and spec changes both matter; status-only CR updates do not.
	depEvent := func(obj any) {
		key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
		if err != nil {
			return
		}
		_, exists, err := b.Intents.Informer().GetIndexer().GetByKey(key)
		if err == nil && exists {
			c.queue.Add(key)
		}
	}
	if _, err := b.Deployments.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: depEvent, UpdateFunc: func(_, n any) { depEvent(n) }, DeleteFunc: depEvent,
	}); err != nil {
		return nil, err
	}
	hpaEvent := func(obj any) {
		if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			obj = tomb.Obj
		}
		h, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
		if !ok {
			return
		}
		if h.Spec.ScaleTargetRef.Kind == "Deployment" {
			c.queue.Add(h.Namespace + "/" + h.Spec.ScaleTargetRef.Name)
		}
	}
	if _, err := b.HPAs.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: hpaEvent, UpdateFunc: func(old, new any) { hpaEvent(old); hpaEvent(new) }, DeleteFunc: hpaEvent,
	}); err != nil {
		return nil, err
	}
	return c, nil
}
func (c *Controller) Run(ctx context.Context) {
	if !cache.WaitForCacheSync(ctx.Done(), c.backend.Deployments.Informer().HasSynced, c.backend.Intents.Informer().HasSynced, c.backend.HPAs.Informer().HasSynced) {
		return
	}
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for c.process(ctx) {
			}
		}()
	}
	<-ctx.Done()
	c.queue.ShutDown()
	workers.Wait()
}
func (c *Controller) process(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	target, err := model.ParseKey(key)
	if err != nil {
		c.queue.Forget(key)
		return true
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = c.engine.Reconcile(callCtx, target)
	cancel()
	if ctx.Err() != nil {
		c.queue.Forget(key)
		return false
	}
	if err != nil {
		c.log.Error("reconcile failed", "target", key, "code", model.ErrorCode(err), "error", err)
		if c.queue.NumRequeues(key) < 8 {
			c.queue.AddRateLimited(key)
		} else {
			c.queue.Forget(key)
			c.queue.AddAfter(key, time.Minute)
		}
	} else {
		c.queue.Forget(key)
		// A modest resync also recovers a missed event and rechecks blocked intent.
		// Deleted intents do not requeue forever.
		if _, exists, _ := c.backend.Intents.Informer().GetIndexer().GetByKey(key); exists {
			c.queue.AddAfter(key, 30*time.Second)
		}
	}
	return true
}
