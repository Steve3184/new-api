package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type hCaptchaCheckResponse struct {
	Success bool `json:"success"`
}

var hCaptchaVerifyURL = "https://api.hcaptcha.com/siteverify"
var hCaptchaHTTPClient = &http.Client{Timeout: 10 * time.Second}

func HCaptchaCheck() gin.HandlerFunc {
	return hCaptchaCheck(true)
}

func HCaptchaCheckFresh() gin.HandlerFunc {
	return hCaptchaCheck(false)
}

func hCaptchaCheck(cacheSession bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.HCaptchaEnabled {
			c.Next()
			return
		}

		session := sessions.Default(c)
		if cacheSession && session.Get("hcaptcha") != nil {
			c.Next()
			return
		}

		token := c.Query("hcaptcha")
		if token == "" {
			token = c.Query("h-captcha-response")
		}
		if token == "" {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "hCaptcha token 为空"})
			c.Abort()
			return
		}
		if strings.TrimSpace(common.HCaptchaSecretKey) == "" {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "hCaptcha configuration is incomplete"})
			c.Abort()
			return
		}

		form := url.Values{
			"secret":   {common.HCaptchaSecretKey},
			"response": {token},
			"remoteip": {c.ClientIP()},
			"sitekey":  {common.HCaptchaSiteKey},
		}
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, hCaptchaVerifyURL, strings.NewReader(form.Encode()))
		if err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := hCaptchaHTTPClient.Do(req)
		if err != nil {
			common.SysLog(err.Error())
			common.ApiError(c, err)
			c.Abort()
			return
		}
		defer resp.Body.Close()

		var result hCaptchaCheckResponse
		if err := common.DecodeJson(resp.Body, &result); err != nil {
			common.SysLog(err.Error())
			common.ApiError(c, err)
			c.Abort()
			return
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !result.Success {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "hCaptcha 校验失败，请刷新重试！"})
			c.Abort()
			return
		}

		if cacheSession {
			session.Set("hcaptcha", true)
			if err := session.Save(); err != nil {
				common.ApiError(c, err)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
