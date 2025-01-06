package helper

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	v1 "kb/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreatRedis(client client.Client, redisConfig *v1.Redis, podName string) (string, error) {
	// 如果在Finalizers中，则说明已经创建了，就不需要再创建
	if IsExist(podName, redisConfig) {
		return "", nil
	}
	newPod := corev1.Pod{}
	newPod.Name = podName
	newPod.Namespace = redisConfig.Namespace
	newPod.Spec.Containers = []corev1.Container{
		{
			//Name:            redisConfig.Name,
			Name:            podName,
			Image:           "redis:5-alpine",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: int32(redisConfig.Spec.Port),
				},
			},
		},
	}
	return podName, client.Create(context.Background(), &newPod)
}

/*
*
根据我们设置的Num，生成指定数量的Pod
*/
func GetPodNameByNum(redisConfig *v1.Redis) []string {
	podNames := make([]string, redisConfig.Spec.Num)
	for i := 0; i < redisConfig.Spec.Num; i++ {
		podNames[i] = fmt.Sprintf("%s-%d", redisConfig.Name, i)
	}
	fmt.Println("podNames:", podNames)
	return podNames
}

/*
* 将创建的pod放入Finalizers中
 */
func IsExist(podName string, redisConfig *v1.Redis) bool {
	for _, pod := range redisConfig.Finalizers {
		if pod == podName {
			return true
		}
	}
	return false
}
