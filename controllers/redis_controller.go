/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"fmt"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"kb/helper"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	myappv1 "kb/api/v1"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=myapp.test.com,resources=redis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=myapp.test.com,resources=redis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=myapp.test.com,resources=redis/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Redis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	redis := myappv1.Redis{}
	if err := r.Get(ctx, req.NamespacedName, &redis); err != nil {
		fmt.Println(err)
	} else {
		// 删除资源时，清除Finalizers
		if !redis.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, r.clearRedis(ctx, &redis)
		}
		// 创建Pod
		podNames := helper.GetPodNameByNum(&redis)
		isEdit := false
		for _, podName := range podNames {
			pName, err := helper.CreatRedis(r.Client, &redis, podName)
			if err != nil {
				return ctrl.Result{}, err
			}
			if pName == "" {
				continue
			}
			redis.Finalizers = append(redis.Finalizers, pName)
			isEdit = true
		}
		// 收缩副本
		if len(redis.Finalizers) > len(podNames) {
			isEdit = true
			err := r.rmIfSurplus(ctx, podNames, &redis)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// 当有新的pod被加入，触发update
		// 执行Update，会再次触发 Reconcile，使用做好幂等性校验
		if isEdit {
			err := r.Client.Update(ctx, &redis)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

/*
*
删除Finalizers中的pod
*/
func (r *RedisReconciler) clearRedis(ctx context.Context, redis *myappv1.Redis) error {
	podList := redis.Finalizers
	for _, podName := range podList {
		err := r.Client.Delete(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: redis.Namespace},
		})
		if err != nil {
			return err
		}
	}
	redis.Finalizers = []string{}
	return r.Client.Update(ctx, redis)
}

/*
*
收缩副本；["redis0","redis1"] --> ["redis0"]
也就是说，我们要删除需要收缩的副本，
之后更新Finalizers

podNames: 要删除的
*/
func (r *RedisReconciler) rmIfSurplus(ctx context.Context, podNames []string, redis *myappv1.Redis) error {
	for i := 0; i < len(redis.Finalizers)-len(podNames); i++ {
		err := r.Client.Delete(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: redis.Finalizers[len(podNames)+i], Namespace: redis.Namespace,
			},
		})
		if err != nil {
			return err
		}
	}
	redis.Finalizers = podNames
	return nil
}

// 监听CR创建出来的POD
// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myappv1.Redis{}).
		Watches(&source.Kind{
			Type: &corev1.Pod{},
		}, &handler.Funcs{DeleteFunc: func(event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
			fmt.Println("pod deleted:", event.Object.GetName())
		}}).
		Complete(r)
}
