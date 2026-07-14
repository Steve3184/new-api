package controller

import "github.com/gin-gonic/gin"

// ThreeDGenerations
// @Summary 创建 3D 生成任务
// @Description 支持文生 3D、图生 3D，以及通过 source_task_id 为已有 draft 生成材质
// @Tags 3D
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ThreeDRequest true "3D 生成请求"
// @Success 200 {object} dto.ThreeDResponse
// @Failure 400 {object} dto.OpenAIError
// @Router /v1/3d [post]
func ThreeDGenerations(c *gin.Context) {}

// ThreeDGenerationTask
// @Summary 查询 3D 生成任务
// @Tags 3D
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "Task ID"
// @Success 200 {object} dto.ThreeDResponse
// @Failure 404 {object} dto.OpenAIError
// @Router /v1/3d/{task_id} [get]
func ThreeDGenerationTask(c *gin.Context) {}
