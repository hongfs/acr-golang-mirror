package main

import (
	"context"
	"errors"
	"fmt"
	cr20160607 "github.com/alibabacloud-go/cr-20160607/v2/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapiutil "github.com/alibabacloud-go/openapi-util/service"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/google/go-github/v47/github"
	"github.com/tidwall/gjson"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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

type Tag struct {
	ID       int64
	Name     string
	LastPush time.Time
	Force    bool
}

func main() {
	tags, err := getDockerTags()

	if err != nil {
		log.Printf("获取 Docker Tags 失败：%s\n", err.Error())

		return
	}

	acr, err := getAliyunAcrTags()

	if err != nil {
		log.Printf("获取 Acr Tags 失败：%s\n", err.Error())
	}

	client := getGitHubClient()

	for _, tag := range tags {
		acrTime, ok := acr[tag.Name]

		// ACR 不存在该版本
		if !ok {
			err := createVersion(client, tag.Name, true)

			if err != nil {
				log.Printf("创建版本异常[%s]：[%s] \n", tag.Name, err.Error())
			}

			continue
		}

		// 完整版本号的则不需要管
		// 意味下面处理的是 1.18 这种不具体版本的
		if len(strings.Split(tag.Name, ".")) >= 2 {
			continue
		}

		// 一个小时内创建过的不管
		if acrTime.Add(-1 * time.Hour).After(tag.LastPush) {
			continue
		}

		err := createVersion(client, tag.Name, tag.Force)

		if err != nil {
			log.Printf("创建版本异常[%s]：[%s] \n", tag.Name, err.Error())
		}

		continue
	}
}

// getDockerToken 获取 Docker 令牌，提升接口访问额度
func getDockerToken(username, password string) (string, error) {
	values := url.Values{}
	values.Add("username", username)
	values.Add("password", password)

	resp, err := http.PostForm("https://hub.docker.com/v2/users/login", values)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	token := gjson.GetBytes(body, "token").String()

	if token == "" {
		return "", errors.New("获取 Token 失败")
	}

	return token, nil
}

// getDockerTags 获取 Docker Golang 的版本列表
// 同时去除了一些不必要、旧的版本
func getDockerTags() (list []Tag, err error) {
	token, err := getDockerToken(DockerUsername, DockerPassword)

	if err != nil {
		return nil, err
	}

	var page = 1

	for {
		client := &http.Client{}

		req, err := http.NewRequest("GET", fmt.Sprintf("https://hub.docker.com/v2/repositories/library/golang/tags/?page_size=1000&page=%d", page), nil)

		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)

		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			return nil, err
		}

		result := gjson.GetBytes(body, "results")

		for _, item := range result.Array() {
			if item.Get("tag_status").String() != "active" {
				continue
			}

			name := item.Get("name").String()

			// 匹配出版本号刚刚好是三段全数字的
			ok, _ := regexp.MatchString("(?m)^(\\d{1,2}\\.)?(\\d{1,2}\\.)?(\\d{1,2})$", name)

			if !ok {
				continue
			}

			// 去除低于 1.15 版本
			if strings.HasPrefix(name, "1.") {
				minor, err := strconv.Atoi(strings.Split(name, ".")[1])

				if err != nil {
					continue
				}

				if minor < 15 {
					continue
				}
			}

			for _, version := range []string{"", "-alpine"} {
				var tag = Tag{
					ID:       item.Get("id").Int(),
					Name:     name + version,
					LastPush: item.Get("tag_last_pushed").Time(),
					Force:    len(strings.Split(name, ".")) < 3,
				}

				list = append(list, tag)
			}
		}

		if gjson.GetBytes(body, "next").String() == "" {
			break
		}

		page = page + 1
	}

	return list, nil
}

// getGitHubClient 获取 GitHub 客户端实例
func getGitHubClient() *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: GitHubToken,
		},
	)

	tc := oauth2.NewClient(context.Background(), ts)

	client := github.NewClient(tc)

	baseURL, _ := url.Parse("https://api.github.com/")

	client.BaseURL = baseURL

	return client
}

// getAliyunAcrClient 获取阿里云 ACR 客户端实例
func getAliyunAcrClient() (*cr20160607.Client, error) {
	return cr20160607.NewClient(&openapi.Config{
		AccessKeyId:     tea.String(AcrAccessKeyId),
		AccessKeySecret: tea.String(AcrAccessKeySecret),
		Endpoint:        tea.String(AcrEndpoint),
	})
}

// getAliyunAcrTags 获取阿里云 ACR 已有版本（且是成功正常可用的）
func getAliyunAcrTags() (map[string]time.Time, error) {
	list := make(map[string]time.Time)

	client, err := getAliyunAcrClient()

	if err != nil {
		return nil, err
	}

	query := map[string]interface{}{}
	query["Page"] = 1
	query["PageSize"] = 100

	headers := make(map[string]*string)

	for {
		req := &openapi.OpenApiRequest{
			Headers: headers,
			Query:   openapiutil.Query(query),
		}

		params := &openapi.Params{
			Action:      tea.String("GetRepoTags"),
			Version:     tea.String("2016-06-07"),
			Protocol:    tea.String("HTTPS"),
			Pathname:    tea.String("/repos/" + AcrOwner + "/" + AcrRepo + "/tags"),
			Method:      tea.String("GET"),
			AuthType:    tea.String("AK"),
			Style:       tea.String("ROA"),
			ReqBodyType: tea.String("json"),
			BodyType:    tea.String("string"),
		}

		resp, err := client.CallApi(params, req, &util.RuntimeOptions{})

		if err != nil {
			return nil, err
		}

		body, ok := resp["body"]

		if !ok {
			return nil, errors.New("获取 Body 失败")
		}

		data := gjson.Get(body.(string), "data.tags")

		for _, item := range data.Array() {
			name := item.Get("tag").String()

			if strings.HasPrefix(name, "__ACR_") {
				continue
			}

			if item.Get("status").String() != "NORMAL" {
				continue
			}

			list[name] = time.UnixMilli(item.Get("imageUpdate").Int())
		}

		if len(data.Array()) != query["PageSize"].(int) {
			break
		}

		query["Page"] = query["Page"].(int) + 1
	}

	return list, nil
}

// createVersion 往 GitHub 上创建新版本
func createVersion(client *github.Client, version string, force bool) error {
	log.Printf("创建新版本：%s\n", version)

	ctx := context.Background()

	content, _, _, err := client.Repositories.GetContents(ctx, GitHubOwner, GitHubRepo, DockerFile, nil)

	sha := ""

	if err == nil {
		sha = content.GetSHA()
	}

	resp, _, err := client.Repositories.UpdateFile(ctx, GitHubOwner, GitHubRepo, DockerFile, &github.RepositoryContentFileOptions{
		Message: github.String("auto-" + version),
		Content: []byte("FROM golang:" + version + "\n"),
		SHA:     github.String(sha),
	})

	if err != nil {
		return err
	}

	tagName := github.String("release-v" + version)

	_time := time.Now()

	_, _, err = client.Git.CreateTag(ctx, GitHubOwner, GitHubRepo, &github.Tag{
		Tag:     tagName,
		Message: tagName,
		Object: &github.GitObject{
			SHA:  github.String(resp.GetSHA()),
			Type: github.String("commit"),
			URL:  github.String(resp.GetURL()),
		},
		Tagger: &github.CommitAuthor{
			Name:  github.String(GitHubOwner),
			Email: github.String("hong@hongfs.cn"),
			Date:  &_time,
		},
		Verification: &github.SignatureVerification{
			Verified: github.Bool(false),
		},
	})

	if err != nil {
		return err
	}

	if force {
		_, _ = client.Git.DeleteRef(ctx, GitHubOwner, GitHubRepo, "refs/tags/release-v"+version)
	}

	_, _, _ = client.Git.CreateRef(ctx, GitHubOwner, GitHubRepo, &github.Reference{
		Ref: github.String("refs/tags/release-v" + version),
		Object: &github.GitObject{
			SHA:  github.String(resp.GetSHA()),
			Type: github.String("commit"),
			URL:  github.String(resp.GetURL()),
		},
	})

	return nil
}
