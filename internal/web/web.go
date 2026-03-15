package web

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

//go:embed all:html
var webRoot embed.FS

type WebHostingOptions struct {
	DevMode       bool
	StaticHosting bool
	UIProxyUrl    string
	Engine        *gin.Engine
}

var etagMap map[string]string

func RegisterUI(options *WebHostingOptions) error {
	if options.StaticHosting {
		log.Info().Msg("Hosting embedded static files for UI")
		sub, err := fs.Sub(webRoot, "html")
		if err != nil {
			return fmt.Errorf("error getting subdirectory for webRoot: %w", err)
		}
		etagMap = buildETagMap(sub)
		options.Engine.Use(gzip.Gzip(gzip.DefaultCompression))
		options.Engine.Use(getEmbeddedFileHandler(sub))
	} else {
		log.Info().Msg("Setting up reverse proxy for UI")
		options.Engine.Use(getProxyHandler(options))
	}
	return nil
}

func buildETagMap(fsys fs.FS) map[string]string {
	etags := make(map[string]string)
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		f, err := fsys.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return nil
		}
		etags["/"+p] = fmt.Sprintf(`W/"%x"`, h.Sum(nil))
		return nil
	})
	return etags
}

func isHashedAsset(urlPath string) bool {
	dir := path.Dir(urlPath)
	return strings.HasPrefix(dir, "/assets")
}

func getEmbeddedFileHandler(sub fs.FS) gin.HandlerFunc {
	readFile := func(p string) ([]byte, error) {
		p = strings.TrimPrefix(p, "/")
		file, err := sub.Open(p)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(file)
	}

	indexFileData, err := readFile("index.html")
	if err != nil {
		indexFileData = []byte("<html><body><h1>Rabbithole</h1><p>UI not built yet.</p></body></html>")
	}

	fileServer := http.FileServer(http.FS(sub))

	return gin.HandlerFunc(func(c *gin.Context) {
		reqPath := c.Request.URL.Path

		if strings.HasPrefix(reqPath, "/api/") {
			c.Next()
			return
		}

		servingIndex := false
		_, err := readFile(reqPath)
		if err != nil {
			servingIndex = true
			reqPath = "/index.html"
		}

		if etag, ok := etagMap[reqPath]; ok {
			c.Header("ETag", etag)
			if match := c.GetHeader("If-None-Match"); match == etag {
				c.Status(http.StatusNotModified)
				c.Abort()
				return
			}
		}

		if reqPath == "/index.html" || servingIndex {
			c.Header("Cache-Control", "no-cache")
		} else if isHashedAsset(reqPath) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "public, max-age=86400")
		}

		if servingIndex {
			c.Data(http.StatusOK, "text/html", indexFileData)
		} else {
			fileServer.ServeHTTP(c.Writer, c.Request)
		}
		c.Abort()
	})
}

func getProxyHandler(options *WebHostingOptions) gin.HandlerFunc {
	proxyUrl, err := url.Parse(options.UIProxyUrl)
	if err != nil {
		log.Panic().Err(err).Msgf("unable to parse target url '%s'", options.UIProxyUrl)
	}
	proxy := httputil.NewSingleHostReverseProxy(proxyUrl)
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
