# Go CI/CD GitOps Demo

这是一个基于 Go、Jenkins、BuildKit、Harbor、Trivy、Argo CD 和 Kubernetes 构建的端到端 CI/CD 与 GitOps实验项目。

## 项目架构

    GitHub Source Repository
              |
              v
    Jenkins Multibranch Pipeline
              |
              v
       Go Test / Go Build
              |
              v
       BuildKit Build Image
              |
              v
        Harbor Registry
              |
              v
       Trivy Security Gate
              |
              v
       GitOps Repository
              |
              v
            Argo CD
              |
              v
          Kubernetes
              |
              v
          Smoke Test

Jenkins负责持续集成、安全扫描和更新GitOps仓库；Argo CD负责持续交付，不再由Jenkins直接修改Deployment。

## 技术栈

| 组件 | 作用 |
|---|---|
| Go | HTTP示例应用和Harbor扫描门禁程序 |
| GitHub | 保存应用源码和GitOps仓库 |
| Jenkins | 执行多分支流水线 |
| Kubernetes Plugin | 创建临时Jenkins Agent Pod |
| BuildKit Rootless | 构建并推送容器镜像 |
| Harbor | 私有镜像仓库 |
| Trivy | 容器镜像漏洞扫描 |
| Argo CD | GitOps持续交付 |
| Kubernetes | 运行应用 |
| Helm | 安装Jenkins、Harbor等平台组件 |

## 两个GitHub仓库

应用源码仓库：

    https://github.com/suellenmorrissey2461986jxe-maker/go-cicd-demo

GitOps状态仓库：

    https://github.com/suellenmorrissey2461986jxe-maker/go-cicd-demo-gitops

应用仓库保存源码、Jenkinsfile、Dockerfile、Harbor扫描门禁和Kubernetes配置。

GitOps仓库保存Argo CD监控的Deployment期望状态。Jenkins只更新镜像Digest，Argo CD负责同步到集群。

## 应用接口

应用监听8080端口：

| 接口 | 预期响应 | 用途 |
|---|---|---|
| `/` | `Hello Kubernetes CI/CD` | 业务测试 |
| `/healthz` | `ok` | 存活和就绪探针 |
| `/version` | `git-<commit>` | 验证部署版本 |

## Jenkins流水线

流水线包含以下阶段：

1. **Checkout**
   - 拉取当前Git提交。
   - 生成`git-<12位commit>`镜像标签。
   - 清理上一次构建的临时文件。

2. **Validate GitOps Credential**
   - 校验GitOps SSH私钥格式。
   - 使用`git ls-remote`验证仓库访问权限。

3. **Go Test**
   - 执行应用和Harbor门禁程序的单元测试。

4. **Go Build**
   - 编译Go应用。
   - 将Git版本写入`/version`接口。

5. **Build and Push Image**
   - 使用Rootless BuildKit构建镜像。
   - 将镜像和构建缓存推送到Harbor。

6. **Harbor Scan Gate**
   - 获取镜像Digest。
   - 等待Trivy扫描完成。
   - 拒绝包含High或Critical漏洞的镜像。
   - 扫描通过后生成不可变镜像引用。

7. **Update GitOps Repository**
   - 修改GitOps仓库中的Deployment镜像Digest。
   - 创建Git提交并推送到main分支。

8. **Deploy to Kubernetes**
   - 这是旧的Jenkins直连部署阶段。
   - 当前已永久关闭，实际部署交给Argo CD。

9. **Smoke Test**
   - 请求Argo CD Hard Refresh。
   - 等待目标镜像Digest应用到Deployment。
   - 等待滚动发布完成。
   - 检查根接口、健康接口和版本接口。

## Harbor安全门禁

镜像仓库：

    100.113.248.106:30002/go-cicd-demo/go-cicd-demo

当前门禁标准：

    Critical = 0
    High     = 0

部署使用不可变Digest：

    repository@sha256:<digest>

如果扫描发现High或Critical漏洞，流水线立即失败，不会更新GitOps仓库。

## Jenkins构建参数

| 参数 | 默认值 | 作用 |
|---|---:|---|
| `DEPLOY_AFTER_BUILD` | `true` | 扫描通过后更新GitOps并执行Smoke Test |
| `FORCE_SMOKE_FAILURE` | `false` | 主动制造Smoke Test失败 |

部署构建必须满足：

- `DEPLOY_AFTER_BUILD=true`
- 当前分支为`main`
- Harbor扫描通过
- 镜像Digest有效

## Kubernetes部署策略

Deployment包含：

- 两个应用副本
- RollingUpdate滚动发布
- `maxUnavailable: 0`
- `maxSurge: 1`
- Readiness Probe
- Liveness Probe
- CPU和内存限制
- 非Root运行
- RuntimeDefault Seccomp
- 只读根文件系统
- 删除全部Linux Capability
- Pod反亲和
- 拓扑分散约束

两个应用Pod分别调度到不同工作节点，提高节点故障下的可用性。

## RBAC最小权限

`k8s/jenkins-rbac.yaml`为Jenkins Agent提供实验所需的Kubernetes权限。

`k8s/jenkins-argocd-refresh-rbac.yaml`只允许Jenkins对`go-cicd-demo` Application执行`get`和`patch`，用于请求Argo CD刷新。

仓库不保存真实密码、Token、SSH私钥和Secret明文。

## 部署验证

查看Argo CD状态：

    kubectl get application go-cicd-demo -n argocd

查看Deployment镜像：

    kubectl get deployment go-cicd-demo -n go-cicd-demo \
      -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'

查看Pod分布：

    kubectl get pods -n go-cicd-demo -l app=go-cicd-demo -o wide

验证服务：

    curl http://100.113.248.106:31042/
    curl http://100.113.248.106:31042/healthz
    curl http://100.113.248.106:31042/version

## 已完成验证

- Go单元测试通过
- BuildKit镜像构建成功
- 镜像成功推送Harbor
- Trivy扫描成功
- High和Critical漏洞均为0
- Jenkins成功更新GitOps仓库
- Argo CD成功同步最新Revision
- Deployment使用通过扫描的镜像Digest
- 两个Pod分布在不同工作节点
- Kubernetes滚动发布成功
- 三个Smoke Test接口验证成功
- Jenkins流水线最终状态为SUCCESS

## 主要排障记录

- Jenkins访问GitHub时的代理和`NO_PROXY`问题。
- 参数化构建实际值为false导致部署阶段跳过。
- Harbor Core和Jobservice异常导致镜像推送502。
- Trivy扫描超时及Harbor数据库恢复状态问题。
- GitOps SSH私钥格式及凭据校验问题。
- Argo CD未及时发现新提交的问题。
- Jenkins缺少Argo CD Application Patch权限。
- Smoke Test执行早于Argo CD同步的问题。
- Jenkins直连部署与Argo CD GitOps职责冲突的问题。

## 项目目录

    .
    ├── cmd/harbor-gate/
    ├── docs/
    ├── gitops/
    ├── k8s/
    ├── Dockerfile
    ├── Jenkinsfile
    ├── go.mod
    ├── main.go
    ├── main_test.go
    └── README.md

## 实验结论

本项目完成了从代码提交、测试、镜像构建、安全扫描、GitOps变更、Argo CD部署到上线验证的完整闭环。

记住这条主线：

    Jenkins负责CI和发布GitOps变更
    Argo CD负责CD和维护Kubernetes期望状态
