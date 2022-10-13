# ACR Golang 镜像同步

本工具用于买不起 [阿里云容器镜像服务](https://www.aliyun.com/product/acr) 企业版而只能被迫选择个人版的同学。一定程度上减少了频繁构建时来自 [Docker 限流](https://www.docker.com/increase-rate-limits/) 异常。

## 配置

最开始你要提供一些配置才可以让程序满足运行条件。

`main.go`

```golang
const (
    // DockerUsername Docker 用户名
    DockerUsername = ""
    // DockerPassword Docker 密码
    DockerPassword = ""
    DockerFile     = "Dockerfile"
    
    // GitHubToken GitHub 令牌
    GitHubToken = ""
    // GitHubOwner GitHub 命名空间名称
    GitHubOwner = "hongfs"
    // GitHubRepo GitHub 仓库名称
    GitHubRepo = "golang"
    
    // AcrAccessKeyId 阿里云AK
    AcrAccessKeyId = ""
    // AcrAccessKeySecret 阿里云AS
    AcrAccessKeySecret = ""
    // AcrEndpoint ACR 地域节点
    AcrEndpoint = "cr.cn-shenzhen.aliyuncs.com"
    // AcrOwner ACR 命名空间名称
    AcrOwner = "hongfs"
    // AcrRepo ACR 仓库名称
    AcrRepo = "golang"
)
```

## 使用

> 使用前，请确保你的环境可以正常访问 `hub.docker.com` `api.github.com`。

配置完后就可以直接进行打包了，后期的更新则需要你自行设置定时任务，建议定时任务为每天执行一次即可。

## License

MIT
