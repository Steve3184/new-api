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
	threeDContentRouter.Use(middleware.TokenOrUserAuth())
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
		videoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
	}

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.POST("/video/generations", controller.RelayTask)
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
	}
	// openai compatible API video routes
	// docs: https://platform.openai.com/docs/api-reference/videos/create
	{
		videoV1Router.POST("/videos", controller.RelayTask)
		videoV1Router.GET("/videos/:task_id", controller.RelayTaskFetch)
	}

	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", controller.RelayTask)
		klingV1Router.POST("/videos/image2video", controller.RelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", controller.RelayTaskFetch)
		klingV1Router.GET("/videos/image2video/:task_id", controller.RelayTaskFetch)
	}

	// Jimeng official API routes - direct mapping to official API format
	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		// Maps to: /?Action=CVSync2AsyncSubmitTask&Version=2022-08-31 and /?Action=CVSync2AsyncGetResult&Version=2022-08-31
		jimengOfficialGroup.POST("/", controller.RelayTask)
	}
}
