/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type DefaultValuesResp struct {
	DefaultValues string `json:"defaultValues"`
}

func GetRenderCharts(productName, envName, serviceName string, log *zap.SugaredLogger) ([]*commonservice.RenderChartArg, error) {

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName, "")

	opt := &commonrepo.RenderSetFindOption{
		Name: renderSetName,
	}
	rendersetObj, existed, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return nil, err
	}

	if !existed {
		return nil, nil
	}

	ret := make([]*commonservice.RenderChartArg, 0)

	matchedRenderChartModels := make([]*template.RenderChart, 0)
	if len(serviceName) == 0 {
		matchedRenderChartModels = rendersetObj.ChartInfos
	} else {
		serverList := strings.Split(serviceName, ",")
		stringSet := sets.NewString(serverList...)
		for _, singleChart := range rendersetObj.ChartInfos {
			if !stringSet.Has(singleChart.ServiceName) {
				continue
			}
			matchedRenderChartModels = append(matchedRenderChartModels, singleChart)
		}
	}

	for _, singleChart := range matchedRenderChartModels {
		rcaObj := new(commonservice.RenderChartArg)
		rcaObj.LoadFromRenderChartModel(singleChart)
		rcaObj.EnvName = envName
		ret = append(ret, rcaObj)
	}
	return ret, nil
}

func GetDefaultValues(productName, envName string, log *zap.SugaredLogger) (*DefaultValuesResp, error) {
	renderSetName := commonservice.GetProductEnvNamespace(envName, productName, "")

	ret := &DefaultValuesResp{}

	opt := &commonrepo.RenderSetFindOption{
		Name: renderSetName,
	}
	rendersetObj, existed, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return nil, err
	}

	if !existed {
		return ret, nil
	}
	ret.DefaultValues = rendersetObj.DefaultValues
	return ret, nil
}

// validate yaml content
func validateYamlContent(yamlContent string) error {
	tMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(yamlContent), &tMap); err != nil {
		if err != nil {
			return errors.New("yaml content illegal")
		}
	}
	return nil
}

func generateValuesYaml(yamlData *commonservice.YamlData, log *zap.SugaredLogger) error {
	if yamlData == nil {
		return nil
	}
	if yamlData.YamlSource == setting.ValuesYamlSourceFreeEdit {
		return validateYamlContent(yamlData.ValuesYAML)
	} else if yamlData.YamlSource == setting.ValuesYamlSourceGitRepo {
		if yamlData.GitRepoConfig == nil {
			return errors.New("invalid repo config")
		}
		var (
			allValues      []byte
			fileContentMap sync.Map
			wg             sync.WaitGroup
			err            error
		)

		for index, filePath := range yamlData.GitRepoConfig.ValuesPaths {
			wg.Add(1)
			go func(index int, path string) {
				defer wg.Done()
				fileContent, err1 := fsservice.DownloadFileFromSource(
					&fsservice.DownloadFromSourceArgs{
						CodehostID: yamlData.GitRepoConfig.CodehostID,
						Owner:      yamlData.GitRepoConfig.Owner,
						Repo:       yamlData.GitRepoConfig.Repo,
						Path:       path,
						Branch:     yamlData.GitRepoConfig.Branch,
					})
				if err1 != nil {
					err = errors.Errorf("fail to download file from git, err: %s, path: %s, repo: %v", err1.Error(), path, *yamlData.GitRepoConfig)
					return
				}
				fileContentMap.Store(index, fileContent)
			}(index, filePath)
		}
		wg.Wait()

		if err != nil {
			return err
		}

		allValueYamls := make([][]byte, len(yamlData.GitRepoConfig.ValuesPaths), len(yamlData.GitRepoConfig.ValuesPaths))
		for i := 0; i < len(yamlData.GitRepoConfig.ValuesPaths); i++ {
			contentObj, _ := fileContentMap.Load(i)
			allValueYamls[i] = contentObj.([]byte)
		}
		allValues, err = yamlutil.Merge(allValueYamls)
		if err != nil {
			return errors.Errorf("failed to merge yaml files, repo: %v", *yamlData.GitRepoConfig)
		}
		yamlData.ValuesYAML = string(allValues)
	}
	return nil
}

func CreateOrUpdateRenderset(productName, envName string, args *commonservice.RendersetArg, userName, requestID string, log *zap.SugaredLogger) error {
	serviceName := args.ServiceName

	serviceOpt := &commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
		Type:        setting.HelmDeployType,
	}
	serviceObj, err := commonrepo.NewServiceColl().Find(serviceOpt)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(fmt.Sprintf("failed to get service %s", err.Error()))
	}
	if serviceObj == nil {
		return e.ErrCreateRenderSet.AddDesc("service not found")
	}

	if serviceObj.HelmChart == nil {
		return e.ErrCreateRenderSet.AddDesc("missing helm chart info")
	}

	if args.RenderChartArg != nil && args.RenderChartArg.YamlData != nil {
		err := generateValuesYaml(args.RenderChartArg.YamlData, log)
		if err != nil {
			return e.ErrCreateRenderSet.AddDesc(err.Error())
		}
	}
	if args.DefaultValues != nil && args.DefaultValues.YamlSource != "" {
		err := generateValuesYaml(args.DefaultValues, log)
		if err != nil {
			return e.ErrCreateRenderSet.AddDesc(err.Error())
		}
	}

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName, "")

	opt := &commonrepo.RenderSetFindOption{Name: renderSetName}
	curRenderset, found, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(fmt.Sprintf("failed to get renderset %s", err.Error()))
	}

	if !found {
		curRenderset = &commonmodels.RenderSet{
			Name:        renderSetName,
			EnvName:     envName,
			ProductTmpl: productName,
			UpdateBy:    userName,
			IsDefault:   false,
		}
	}

	//update or insert service values.yaml
	if args.RenderChartArg != nil {
		var targetChartInfo *template.RenderChart
		for _, singleChart := range curRenderset.ChartInfos {
			if singleChart.ServiceName != serviceName {
				continue
			}
			args.RenderChartArg.FillRenderChartModel(singleChart, singleChart.ChartVersion)
			targetChartInfo = singleChart
			break
		}
		if targetChartInfo == nil {
			targetChartInfo = new(template.RenderChart)
			targetChartInfo.ValuesYaml = serviceObj.HelmChart.ValuesYaml
			args.FillRenderChartModel(targetChartInfo, serviceObj.HelmChart.Version)
			curRenderset.ChartInfos = append(curRenderset.ChartInfos, targetChartInfo)
		}
	}

	//update environment's defaults.yaml
	if args.DefaultValues != nil {
		curRenderset.DefaultValues = args.DefaultValues.ValuesYAML
	} else {
		curRenderset.DefaultValues = ""
	}

	//create new renderset with increased revision
	err = commonservice.CreateHelmRenderSet(
		curRenderset,
		log,
	)

	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}
	return err
}

func CreateOrUpdateChartValues(productName, envName string, args *commonservice.RenderChartArg, userName, requestID string, log *zap.SugaredLogger) error {

	serviceName := args.ServiceName

	serviceOpt := &commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
		Type:        setting.HelmDeployType,
	}
	serviceObj, err := commonrepo.NewServiceColl().Find(serviceOpt)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(fmt.Sprintf("failed to get service %s", err.Error()))
	}
	if serviceObj == nil {
		return e.ErrCreateRenderSet.AddDesc("service not found")
	}

	if serviceObj.HelmChart == nil {
		return e.ErrCreateRenderSet.AddDesc("missing helm chart info")
	}

	err = generateValuesYaml(args.YamlData, log)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName, "")

	opt := &commonrepo.RenderSetFindOption{Name: renderSetName}
	curRenderset, found, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(fmt.Sprintf("failed to get renderset %s", err.Error()))
	}

	if !found {
		curRenderset = &commonmodels.RenderSet{
			Name:        renderSetName,
			EnvName:     envName,
			ProductTmpl: productName,
			UpdateBy:    userName,
			IsDefault:   false,
		}
	}

	//update or insert service values.yaml
	var targetChartInfo *template.RenderChart
	for _, singleChart := range curRenderset.ChartInfos {
		if singleChart.ServiceName != serviceName {
			continue
		}
		args.FillRenderChartModel(singleChart, singleChart.ChartVersion)
		targetChartInfo = singleChart
		break
	}
	if targetChartInfo == nil {
		targetChartInfo = new(template.RenderChart)
		targetChartInfo.ValuesYaml = serviceObj.HelmChart.ValuesYaml
		args.FillRenderChartModel(targetChartInfo, serviceObj.HelmChart.Version)
		curRenderset.ChartInfos = append(curRenderset.ChartInfos, targetChartInfo)
	}

	//create new renderset with increased revision
	err = commonservice.CreateHelmRenderSet(
		curRenderset,
		log,
	)

	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}
	return err
}

func ensureHelmRenderSetArgs(args *commonmodels.RenderSet) error {
	if args == nil {
		return errors.New("nil RenderSet")
	}

	if len(args.Name) == 0 {
		return errors.New("empty render set name")
	}
	// 设置新的版本号
	rev, err := commonrepo.NewCounterColl().GetNextSeq("renderset:" + args.Name)
	if err != nil {
		return fmt.Errorf("get next render set revision error: %v", err)
	}

	args.Revision = rev
	return nil
}
