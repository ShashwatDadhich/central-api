package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	util "github.com/devtron-labs/central-api/client"
	"github.com/devtron-labs/central-api/common"
	"github.com/google/go-github/github"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"sync"
	"time"
)

type ReleaseNoteService interface {
	GetModules() ([]*common.Module, error)
	GetReleases() ([]*common.Release, error)
	UpdateReleases(requestBodyBytes []byte) (bool, error)
}

type ReleaseNoteServiceImpl struct {
	logger       *zap.SugaredLogger
	client       *util.GitHubClient
	releaseCache *util.ReleaseCache
	mutex        sync.Mutex
}

func NewReleaseNoteServiceImpl(logger *zap.SugaredLogger, client *util.GitHubClient, releaseCache *util.ReleaseCache) *ReleaseNoteServiceImpl {
	serviceImpl := &ReleaseNoteServiceImpl{
		logger:       logger,
		client:       client,
		releaseCache: releaseCache,
	}
	_, err := serviceImpl.GetReleases()
	if err != nil {
		serviceImpl.logger.Errorw("error on app init call for releases", "err", err)
		//ignore error for starting application
	}
	return serviceImpl
}

const ActionPublished = "published"
const EventTypeRelease = "release"

func (impl *ReleaseNoteServiceImpl) UpdateReleases(requestBodyBytes []byte) (bool, error) {
	data := make(map[string]interface{})
	err := json.Unmarshal(requestBodyBytes, &data)
	if err != nil {
		impl.logger.Errorw("unmarshal error", "err", err)
		return false, err
	}
	action := data["action"].(string)
	if action != ActionPublished {
		return false, nil
	}
	releaseData := data["release"].(map[string]interface{})
	releaseName := releaseData["name"].(string)
	tagName := releaseData["tag_name"].(string)
	createdAtString := releaseData["created_at"].(string)
	createdAt, error := time.Parse("2006-01-02T15:04:05.000Z", createdAtString)
	if error != nil {
		impl.logger.Error(error)
		//return false, nil
	}
	body := releaseData["body"].(string)
	releaseInfo := &common.Release{
		TagName:     tagName,
		ReleaseName: releaseName,
		Body:        body,
		CreatedAt:   createdAt,
	}

	//updating cache, fetch existing object and append new item
	var releaseList []*common.Release
	releaseList = append(releaseList, releaseInfo)
	cachedReleases := impl.releaseCache.GetReleaseCache()
	if cachedReleases != nil {
		itemMap, ok := cachedReleases.(map[string]cache.Item)
		if !ok {
			// Can't assert, handle error.
			impl.logger.Error("Can't assert, handle err")
			return false, nil
		}
		impl.logger.Info(itemMap)
		if itemMap != nil {
			items := itemMap["releases"]
			if items.Object != nil {
				releases := items.Object.([]*common.Release)
				releaseList = append(releaseList, releases...)
			}
		}
	}
	impl.mutex.Lock()
	defer impl.mutex.Unlock()
	impl.releaseCache.UpdateReleaseCache(releaseList)
	return true, nil
}

func (impl *ReleaseNoteServiceImpl) GetReleases() ([]*common.Release, error) {
	var releaseList []*common.Release
	cachedReleases := impl.releaseCache.GetReleaseCache()
	if cachedReleases != nil {
		itemMap, ok := cachedReleases.(map[string]cache.Item)
		if !ok {
			impl.logger.Error("Can't assert, handle err")
			return releaseList, nil
		}
		impl.logger.Info(itemMap)
		if itemMap != nil {
			items := itemMap["releases"]
			if items.Object != nil {
				releases := items.Object.([]*common.Release)
				releaseList = append(releaseList, releases...)
			}
		}
	}

	if releaseList == nil {
		operationComplete := false
		retryCount := 0
		for !operationComplete && retryCount < 3 {
			retryCount = retryCount + 1
			releases, _, err := impl.client.GitHubClient.Repositories.ListReleases(context.Background(), impl.client.GitHubConfig.GitHubOrg, impl.client.GitHubConfig.GitHubRepo, &github.ListOptions{})
			if err != nil {
				responseErr, ok := err.(*github.ErrorResponse)
				if !ok || responseErr.Response.StatusCode != 404 {
					impl.logger.Errorw("error in fetching releases from github", "err", err, "config", "config")
					//todo - any specific message
					continue
				} else {
					impl.logger.Errorw("error in fetching releases from github", "err", err)
					continue
				}
			}
			if err == nil {
				operationComplete = true
			}
			result := &common.ReleaseList{}
			var releasesDto []*common.Release
			for _, item := range releases {
				dto := &common.Release{
					TagName:     *item.TagName,
					ReleaseName: *item.Name,
					CreatedAt:   item.CreatedAt.Time,
					PublishedAt: item.PublishedAt.Time,
					Body:        *item.Body,
				}
				releasesDto = append(releasesDto, dto)
			}
			result.Releases = releasesDto
			releaseList = releasesDto
			impl.mutex.Lock()
			defer impl.mutex.Unlock()
			impl.releaseCache.UpdateReleaseCache(releaseList)
		}
		if !operationComplete {
			return releaseList, fmt.Errorf("failed operation on fetching releases from github, attempted 3 times")
		}
	}

	return releaseList, nil
}

func (impl *ReleaseNoteServiceImpl) GetModules() ([]*common.Module, error) {
	var modules []*common.Module
	modules = append(modules, &common.Module{
		Id:                            1,
		Name:                          common.MODULE_CICD,
		BaseMinVersionSupported:       "v0.0.1",
		IsIncludedInLegacyFullPackage: true,
		Assets:                        []string{"/dashboard/static/media/ic-empty-ea-app-detail.83927d25.png", "/dashboard/static/media/ic-empty-ea-charts.8556c797.png"},
		Description:                   "<div class=\"module-details__feature-info fs-13 fw-4\"><p>Continuous integration (CI) and continuous delivery (CD) embody a culture, set of operating principles, and collection of practices that enable application development teams to deliver code changes more frequently and reliably. The implementation is also known as the CI/CD pipeline.</p><p>CI/CD is one of the best practices for devops teams to implement. It is also an agile methodology best practice, as it enables software development teams to focus on meeting business requirements, code quality, and security because deployment steps are automated.</p><h3 class=\"module-details__features-list-heading fs-13 fw-6\">Features</h3><ul class=\"module-details__features-list pl-22 mb-24\"><li>Discovery: What would the users be searching for when they're looking for a CI/CD offering?</li><li>Detail: The CI/CD offering should be given sufficient importance (on Website, Readme). (Eg. Expand capability with CI/CD module [Discover more modules])</li><li>Installation: Ability to install CI/CD module with the basic installation.</li><li>In-Product discovery: How easy it is to discover the CI/CD offering primarily once the user is in the product. (Should we talk about modules on the login page?)</li></ul></div>",
		Title:                         "Build and Deploy (CI/CD)",
		Icon:                          "data:image/png;base64,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",
		Info:                          "Enables continous code integration and deployment.",
	})
	return modules, nil
}
