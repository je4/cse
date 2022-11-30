package server

import (
	"context"
	"crypto/tls"
	"github.com/Masterminds/sprig"
	"github.com/bluele/gcache"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	dcert "github.com/je4/utils/v2/pkg/cert"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"google.golang.org/api/customsearch/v1"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SearchResort struct {
	Key, Name, Link string
}

type Server struct {
	service            string
	host, port         string
	username, password string
	srv                *http.Server
	linkTokenExp       time.Duration
	jwtKey             string
	jwtAlg             []string
	log                *logging.Logger
	AddrExt            string
	accessLog          io.Writer
	templates          map[string]*template.Template
	httpStaticServer   http.Handler
	staticFS           fs.FS
	templateFS         fs.FS
	templateDev        bool
	search             *customsearch.Service
	resorts            map[string]SearchResort
	cache              gcache.Cache
	domain             map[string]string
}

func NewServer(service, addr, addrExt string,
	staticFS, templateFS fs.FS,
	svc *customsearch.Service, resorts map[string]SearchResort, domain map[string]string, username, password string, templateDev bool, log *logging.Logger, accessLog io.Writer) (*Server, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot split address %s", addr)
	}
	/*
		extUrl, err := url.Parse(addrExt)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse external address %s", addrExt)
		}
	*/

	srv := &Server{
		service:          service,
		search:           svc,
		resorts:          resorts,
		host:             host,
		port:             port,
		staticFS:         staticFS,
		httpStaticServer: http.FileServer(http.FS(staticFS)),
		AddrExt:          strings.TrimRight(addrExt, "/"),
		domain:           domain,
		username:         username,
		password:         password,
		templateDev:      templateDev,
		log:              log,
		accessLog:        accessLog,
		templateFS:       templateFS,
		templates:        map[string]*template.Template{},
		cache:            gcache.New(500).LRU().Expiration(time.Hour * 24).Build(),
	}

	return srv, srv.InitTemplates()
}

func (s *Server) InitTemplates() error {
	entries, err := fs.ReadDir(s.templateFS, ".")
	if err != nil {
		return errors.Wrapf(err, "cannot read template folder %s", "template")
	}
	funcMap := sprig.FuncMap()
	funcMap["urlencode"] = func(str string) string {
		return url.QueryEscape(str)
	}
	funcMap["iterate"] = func(count int) []int {
		var i int
		var Items []int
		for i = 0; i < count; i++ {
			Items = append(Items, i)
		}
		return Items
	}
	for _, entry := range entries {
		name := entry.Name()
		tpl, err := template.New(name).Funcs(funcMap).ParseFS(s.templateFS, name)
		if err != nil {
			return errors.Wrapf(err, "cannot parse template: %s", name)
		}
		s.templates[name] = tpl
	}
	return nil
}

func (s *Server) ListenAndServe(cert, key string) (err error) {
	router := mux.NewRouter()

	router.PathPrefix("/static").Handler(http.StripPrefix("/static", s.httpStaticServer)).Methods("GET")
	router.HandleFunc("/", s.IndexHandler).Methods("GET")
	router.HandleFunc("/search/{resort}", s.SearchHandler).Queries("search", "{search}").Methods("GET")
	router.HandleFunc("/search/{resort}", s.IndexHandler).Queries().Methods("GET")
	loggedRouter := handlers.CombinedLoggingHandler(s.accessLog, handlers.ProxyHeaders(router))
	addr := net.JoinHostPort(s.host, s.port)
	s.srv = &http.Server{
		Handler: loggedRouter,
		Addr:    addr,
	}
	router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathRegexp, err := route.GetPathRegexp()
		if err != nil {
			return errors.Wrapf(err, "cannot get path regexp of route %s", route.GetName())
		}
		queriesRegexp, err := route.GetQueriesRegexp()
		if err != nil {
			return errors.Wrapf(err, "cannot get queries regexp of route %s", route.GetName())
		}
		s.log.Infof("Route %s: %s - %s", route.GetName(), pathRegexp, queriesRegexp)
		return nil
	})
	if cert == "auto" || key == "auto" {
		s.log.Info("generating new certificate")
		cert, err := dcert.DefaultCertificate()
		if err != nil {
			return errors.Wrap(err, "cannot generate default certificate")
		}
		s.srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*cert}}
		s.log.Infof("starting salon digital at %v - https://%s:%v/", s.AddrExt, s.host, s.port)
		return s.srv.ListenAndServeTLS("", "")
	} else if cert != "" && key != "" {
		s.log.Infof("starting salon digital at %v - https://%s:%v/", s.AddrExt, s.host, s.port)
		return s.srv.ListenAndServeTLS(cert, key)
	} else {
		s.log.Infof("starting salon digital at %v - http://%s:%v/", s.AddrExt, s.host, s.port)
		return s.srv.ListenAndServe()
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
