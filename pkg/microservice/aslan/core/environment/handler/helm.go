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

package handler

import (
	"github.com/gin-gonic/gin"

	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func ListReleases(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")

	args := &service.HelmReleaseQueryArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.ListReleases(args, envName, false, ctx.Logger)
}

func ListProductionReleases(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")

	args := &service.HelmReleaseQueryArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.ListReleases(args, envName, true, ctx.Logger)
}

func GetChartValues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")
	projectName := c.Query("projectName")
	serviceName := c.Query("serviceName")

	ctx.Resp, ctx.Err = commonservice.GetChartValues(projectName, envName, serviceName, false, false)
}

// @Summary Get Production Chart Values
// @Description Get Production Chart Values
// @Tags    environment
// @Accept 	json
// @Produce json
// @Param 	name				path		string									true	"env name"
// @Param 	projectName			query		string									true	"project name"
// @Param 	serviceName			query		string									false	"service name"
// @Param 	releaseName			query		string									false	"release name"
// @Param 	isHelmChartDeploy	query		string									true	"isHelmChartDeploy"
// @Param 	body 				body 		service.SyncCollaborationInstanceArgs 	true 	"body"
// @Success 200 				{object} 	commonservice.ValuesResp
// @Router /api/aslan/environment/production/environments/{name}/helm/values [get]
func GetProductionChartValues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")
	projectName := c.Query("projectName")
	serviceName := c.Query("serviceName")
	isHelmChartDeploy := c.Query("isHelmChartDeploy")
	releaseName := c.Query("releaseName")

	if isHelmChartDeploy == "false" {
		ctx.Resp, ctx.Err = commonservice.GetChartValues(projectName, envName, serviceName, false, true)
	} else {
		ctx.Resp, ctx.Err = commonservice.GetChartValues(projectName, envName, releaseName, true, true)
	}
}

func GetChartInfos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")
	servicesName := c.Query("serviceName")
	projectName := c.Query("projectName")
	ctx.Resp, ctx.Err = service.GetChartInfos(projectName, envName, servicesName, ctx.Logger)
}

func GetImageInfos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")
	projectName := c.Query("projectName")
	servicesName := c.Query("serviceName")
	ctx.Resp, ctx.Err = service.GetImageInfos(projectName, envName, servicesName, ctx.Logger)
}
