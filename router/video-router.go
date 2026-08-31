package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	audioSpeechContentRouter := router.Group("/v1")
	audioSpeechContentRouter.Use(middleware.RouteTag("relay"))
	audioSpeechContentRouter.Use(middleware.TokenOrUserAuth())
	{
		audioSpeechContentRouter.GET("/audio/speech/tasks/:task_id/content", controller.AudioSpeechProxy)
		audioSpeechContentRouter.GET("/audio/speech/tasks/:task_id/timestamps", controller.AudioSpeechTimestampsProxy)
	}

	audioSpeechTaskRouter := router.Group("/v1")
	audioSpeechTaskRouter.Use(middleware.RouteTag("relay"))
	audioSpeechTaskRouter.Use(middleware.SystemPerformanceCheck())
	audioSpeechTaskRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		audioSpeechTaskRouter.POST("/audio/speech/tasks", controller.RelayTask)
		audioSpeechTaskRouter.GET("/audio/speech/tasks/:task_id", controller.RelayTaskFetch)
	}

	threeDContentRouter := router.Group("/v1")
	threeDContentRouter.Use(middleware.RouteTag("relay"))
	threeDContentRouter.Use(middleware.DownloadRateLimit())
	{
		threeDContentRouter.GET("/3d/:task_id/content", controller.ThreeDProxy)
	}

	threeDRouter := router.Group("/v1")
	threeDRouter.Use(middleware.RouteTag("relay"))
	threeDRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		threeDRouter.POST("/3d", controller.RelayTask)
		threeDRouter.GET("/3d/:task_id", controller.RelayTaskFetch)
	}

	// Video proxy: accepts either session auth (dashboard) or token auth (API clients)
	videoProxyRouter := router.Group("/v1")
	videoProxyRouter.Use(middleware.RouteTag("relay"))
	videoProxyRouter.Use(middleware.TokenOrUserAuth())
	{
		if !routeExists(router, "GET", "/v1/videos/:task_id/content") {
			videoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
		}
	}

	videoSharedRouter := router.Group("/v1")
	videoSharedRouter.Use(middleware.RouteTag("relay"))
	videoSharedRouter.Use(middleware.TokenAuth())
	videoSharedRouter.Use(middleware.SystemPerformanceCheck())
	videoSharedRouter.POST(
		"/video/generations",
		middleware.PinTaskPluginEndpoint(),
		middleware.TaskPluginEndpointOnly(middleware.ModelRequestRateLimit()),
		middleware.PrepareTaskPluginEndpoint(),
		middleware.Distribute(),
		func(c *gin.Context) {
			controller.RelayTaskPluginEndpoint(c, controller.RelayTask)
		},
	)

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
	}
}
