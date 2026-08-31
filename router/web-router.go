package router

import (
	"bytes"
	"embed"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

// WebAssets holds the embedded dashboard frontend assets.
type WebAssets struct {
	BuildFS   embed.FS
	IndexPage []byte
}

func setHTMLAttribute(node *html.Node, key, value string) {
	for i := range node.Attr {
		if node.Attr[i].Key == key {
			node.Attr[i].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

func findHTMLElement(root *html.Node, element, attribute, value string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if found != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == element {
			if attribute == "" {
				found = node
				return
			}
			for _, attr := range node.Attr {
				if attr.Key == attribute && attr.Val == value {
					found = node
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func upsertMeta(head *html.Node, attribute, name, content string) {
	node := findHTMLElement(head, "meta", attribute, name)
	if node == nil {
		node = &html.Node{Type: html.ElementNode, Data: "meta"}
		setHTMLAttribute(node, attribute, name)
		head.AppendChild(node)
	}
	setHTMLAttribute(node, "content", content)
}

func setElementText(node *html.Node, content string) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		node.RemoveChild(child)
		child = next
	}
	node.AppendChild(&html.Node{Type: html.TextNode, Data: content})
}

func upsertTitle(head *html.Node, title string) {
	node := findHTMLElement(head, "title", "", "")
	if node == nil {
		node = &html.Node{Type: html.ElementNode, Data: "title"}
		head.AppendChild(node)
	}
	setElementText(node, title)
}

func renderSPAIndex(indexPage []byte, logo, systemName string, meta console_setting.SPAMetaSetting) ([]byte, error) {
	document, err := html.Parse(bytes.NewReader(indexPage))
	if err != nil {
		return nil, err
	}
	head := findHTMLElement(document, "head", "", "")
	if head == nil {
		return nil, nil
	}

	if logo != "" {
		if favicon := findHTMLElement(head, "link", "rel", "icon"); favicon != nil {
			setHTMLAttribute(favicon, "href", logo)
		}
	}
	if systemName != "" {
		upsertTitle(head, systemName)
		upsertMeta(head, "name", "title", systemName)
		upsertMeta(head, "property", "og:title", systemName)
	}
	upsertMeta(head, "name", "description", meta.Description)
	upsertMeta(head, "property", "og:type", meta.OGType)
	upsertMeta(head, "property", "og:description", meta.OGDescription)

	var output bytes.Buffer
	if err = html.Render(&output, document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func SetWebRouter(router *gin.Engine, assets WebAssets, pluginDispatcher gin.HandlerFunc) {
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")

	router.NoRoute(
		pluginDispatcher,
		middleware.RouteTag("web"),
		gzip.Gzip(gzip.DefaultCompression),
		middleware.GlobalWebRateLimit(),
		middleware.Cache(),
		static.Serve("/", frontendFS),
		func(c *gin.Context) {
			if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
				controller.RelayNotFound(c)
				return
			}
			c.Header("Cache-Control", "no-cache")
			common.OptionMapRWMutex.RLock()
			logo := common.Logo
			systemName := common.SystemName
			meta := console_setting.GetSPAMetaSetting()
			common.OptionMapRWMutex.RUnlock()
			indexPage, err := renderSPAIndex(assets.IndexPage, logo, systemName, meta)
			if err != nil || indexPage == nil {
				indexPage = assets.IndexPage
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
		},
	)
}
