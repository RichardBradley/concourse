package web

import (
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/web/indexhandler"
	"github.com/concourse/concourse/web/proxyhandler"
	"github.com/concourse/concourse/web/publichandler"
	"github.com/concourse/concourse/web/robotshandler"
)

func NewHandler(
	logger lager.Logger,
	apiURL *url.URL,
	authURL *url.URL,
	xFrameOptions string,
) (http.Handler, error) {

	apiProxy := proxyhandler.NewApiHandler(logger, apiURL, xFrameOptions)
	authProxy := proxyhandler.NewAuthHandler(logger, authURL)

	publicHandler := publichandler.NewHandler()
	robotsHandler := robotshandler.NewHandler()

	indexHandler, err := indexhandler.NewHandler(logger)
	if err != nil {
		return nil, err
	}

	webMux := http.NewServeMux()
	webMux.Handle("/api/", apiProxy)
	webMux.Handle("/sky/", authProxy)
	webMux.Handle("/auth/", authProxy)
	webMux.Handle("/login", authProxy)
	webMux.Handle("/logout", authProxy)
	webMux.Handle("/public/", publicHandler)
	webMux.Handle("/robots.txt", robotsHandler)
	webMux.Handle("/", indexHandler)
	return webMux, nil
}
