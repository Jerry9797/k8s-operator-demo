# Operator入门

## 环境准备
- go：1.23 
  - https://go.dev/dl/
- kubebuilder
  - https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v3.9.0
- kustomize
  - https://github.com/kubernetes-sigs/kustomize/releases/tag/kustomize%2Fv3.9.2
- mac

## MacOs kubectl本地连接 k8s 集群

- 本地 mac 下载 kubectl，连接远程集群并操作
- macOS 是 Apple Silicon (M1/M2)，下载 ARM 版本
  - curl -LO https://dl.k8s.io/release/v1.23.0/bin/darwin/arm64/kubectl
  - k8s集群开启代理 ,这个代理一般是暴露 8001 端口，让外部访问集群；&是挂载。
  ```
   kubectl proxy --address='0.0.0.0' --accept-hosts='.*' &
   ```
  - 下载集群的配置文件 ~/.kube/config； 修改以下内容
    ```yaml
    clusters:
    - cluster:
        server: http://<服务器IP>:8001
    ```
    
# 使用 kubebuilder 搭建项目开发Operator
做一个 Redis 相关的 Operator, 定义一种新的资源类型（Custom Resource，简称 CR）Kind 为Redis。
```
kubebuilder init --domain test.com
```
- 可能会有网络问题，有些脚本下载不下来，需要使用自己的梯子下载
```
kubebuilder create api --group myapp --version v1 --kind Redis
```
- 查询我们创建的 crd：redis.myapp.test.com
```
kubectl get crd 
```

# 发布到 k8s

## 远程 docker 开启远程访问
- vi /etc/docker/daemon.json  加入以下内容；
  ```
  "hosts": [
    "unix:///var/run/docker.sock",
    "tcp://0.0.0.0:2375"
  ],
  ```
- 执行 
  ```
  sudo systemctl daemon-reload && systemctl restart docker
  ``` 
## 本地 mac 连接远程 docker
- 在你的本地 macOS 上，设置 DOCKER_HOST 环境变量，以连接到远程 Docker 主机，在命令行终端执行：
  ```shell
  export DOCKER_HOST=tcp://<remote-ip>:2375
  ```

## 搭建简易镜像库
- 官方的用于测试的简易镜像库
```
  docker pull registry
  docker run -d --restart=always --name registry -v /Users/heyilu/registry:/var/lib/registry -p 5000:5000 --restart=always --name registry registry
```
- vi /etc/docker/daemon.json 加入以下内容,执行 `sudo systemctl daemon-reload && systemctl restart docker`
  ```
  "insecure-registries": ["<仓库IP>:5000"],
  ```
- vi /usr/lib/sysctl.d/00-system.conf 加入以下内容,执行 `systemctl restart network && systemctl restart docker`
```
  net.ipv4.ip_forward=1
```

## 发布镜像
- `make docker-build docker-push IMG=8.155.39.103:5000/myredis:v1`
- 有些镜像无法直接 pull，mac 本地开梯子 pull，然后上传服务器；需要注意 arm、amd 架构。
- 创建 ns 等： `kubectl apply -k ./config/default`


# 案例开发
## CRD 字段验证
- 校验字段代码（注解的方式）：创建 redis 的 port 最小值为 81, 最大值为 40000
  - 以注释的方式校验，参考 https://book.kubebuilder.io/reference/markers/crd-validation
  - 代码：[redis_types.go](api/v1/redis_types.go)
- 项目 Terminal 1 执行：`make install` 、`make run`
- k8s 查询 crd, 执行`kubectl describe crd redis.myapp.test.com`,可查看到：
  ```
  Port:
      Maximum:  40000
      Minimum:  81
      Type:     integer
  ```
- 本地测试
  ```yaml
  apiVersion: myapp.test.com/v1
  kind: Redis
  metadata:
    name: myredis
  spec:
    port: 101111
  ```
  -  Terminal 2 执行：`kubectl apply -f ./test/redis.yaml` 发布资源
    - Terminal 1 预期得到：The Redis "myredis" is invalid: spec.port: Invalid value: 101111: spec.port in body should be less than or equal to 40000

  
# 发布CR时创建容器

**需求：**
- 在提交资源时，自动创建一个 redis 相关的容器，提交时会自动调用下方方法
- 当执行 `kubectl delete -f ./test/redis.yaml` 删除掉CR: Reids ，以及对应的 pod
- [redis_controller.go](controllers/redis_controller.go) 中的Reconcile方法
  - 在此方法中可以自定义容器 创建、修改、删除 的逻辑
- `make run`
- `kubectl apply -f ./test/redis.yaml`

## 使用 Finalizers 做资源清理
- 在 Kubernetes 中，Finalizer 是资源对象的一部分，用于在资源被删除时执行一些清理操作。当资源的删除请求被发出时，Kubernetes 会先调用 Finalizer 指定的逻辑，直到所有 Finalizer 的操作完成后，资源才会真正被删除。
- 当我们创建资源时，可以将 pod 加入Finalizers 切片，当删除资源时，如果Finalizers 没有处理完，则会阻塞，直到 Finalizer 操作成功完成。
- 假设你希望在 Pod 删除之前执行一些清理操作，你可以为该 Pod 添加 Finalizer。以下是一个示例：
  ```yaml
  apiVersion: v1
  kind: Pod
  metadata:
    name: mypod
    finalizers:
      - example.com/myfinalizer
  spec:
    containers:
      - name: mycontainer
        image: nginx
  ```
- 当 Pod 被删除时：
  1. 删除请求发出, 请求删除该 Pod。
  2. Finalizer 阻止删除：删除操作会被阻止，直到 example.com/myfinalizer 指定的清理操作完成。如果清理操作没有完成，Pod 删除将被挂起。
  3. Kubernetes 会等这些操作完成后，删除 Pod。
  4. 移除 Finalizer 并删除资源：一旦 Finalizer 完成，Kubernetes 会移除 Finalizer，并完成资源的删除。
- 详见 [redis_controller.go](controllers/redis_controller.go) 中的Reconcile方法中的删除 与 收缩逻辑


