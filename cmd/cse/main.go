package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/je4/cse/v2/pkg/server"
	"github.com/je4/cse/v2/web"
	lm "github.com/je4/utils/v2/pkg/logger"
	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var err error

	var basedir = flag.String("basedir", ".", "base folder with html contents")
	var configfile = flag.String("cfg", "/etc/cse.toml", "configuration file")

	flag.Parse()

	var config = &CSEConfig{
		LogFile:   "",
		LogLevel:  "DEBUG",
		LogFormat: `%{time:2006-01-02T15:04:05.000} %{module}::%{shortfunc} [%{shortfile}] > %{level:.5s} - %{message}`,
		BaseDir:   *basedir,
		Addr:      "localhost:80",
		AddrExt:   "http://localhost:80/",
	}
	if err := LoadCSEConfig(*configfile, config); err != nil {
		log.Printf("cannot load config file: %v", err)
	}
	for name, csk := range config.Google.CustomSearchKeys {
		str := os.Getenv(fmt.Sprintf("%s_key", name))
		if str != "" {
			csk.Key = str
		}
	}
	str := os.Getenv("apikey")
	if str != "" {
		config.Google.Apikey = str
	}

	// create logger instance
	logger, lf := lm.CreateLogger("Salon Digital", config.LogFile, nil, config.LogLevel, config.LogFormat)
	defer lf.Close()

	var accessLog io.Writer
	var f *os.File
	if config.AccessLog == "" {
		accessLog = os.Stdout
	} else {
		f, err = os.OpenFile(config.AccessLog, os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			logger.Panicf("cannot open file %s: %v", config.AccessLog, err)
			return
		}
		defer f.Close()
		accessLog = f
	}

	var staticFS, templateFS fs.FS

	if config.StaticDir == "" {
		staticFS, err = fs.Sub(web.StaticFS, "static")
		if err != nil {
			logger.Panicf("cannot get subtree of static: %v", err)
		}
	} else {
		staticFS = os.DirFS(config.StaticDir)
	}

	if config.TemplateDir == "" {
		templateFS, err = fs.Sub(web.TemplateFS, "template")
		if err != nil {
			logger.Panicf("cannot get subtree of embedded template: %v", err)
		}
	} else {
		templateFS = os.DirFS(config.TemplateDir)
	}

	googleSvc, err := customsearch.NewService(context.Background(), option.WithAPIKey(config.Google.Apikey))
	if err != nil {
		log.Panic(err)
	}

	resorts := map[string]server.SearchResort{}
	for n, k := range config.Google.CustomSearchKeys {
		resorts[n] = server.SearchResort{
			Key:  k.Key,
			Name: k.Name,
			Link: k.Link,
		}
	}
	srv, err := server.NewServer(
		"Basel Collection Search",
		config.Addr,
		config.AddrExt,
		staticFS,
		templateFS,
		googleSvc,
		resorts,
		config.Domain,
		config.User,
		config.Password,
		config.TemplateDev,
		logger,
		accessLog,
	)
	if err != nil {
		logger.Panicf("cannot initialize server: %v", err)
	}
	go func() {
		if err := srv.ListenAndServe(config.CertPem, config.KeyPem); err != nil {
			log.Fatalf("server died: %v", err)
		}
	}()

	end := make(chan bool, 1)

	// process waiting for interrupt signal (TERM or KILL)
	go func() {
		sigint := make(chan os.Signal, 1)

		// interrupt signal sent from terminal
		signal.Notify(sigint, os.Interrupt)

		signal.Notify(sigint, syscall.SIGTERM)
		signal.Notify(sigint, syscall.SIGKILL)

		<-sigint

		// We received an interrupt signal, shut down.
		logger.Infof("shutdown requested")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.Shutdown(ctx)

		end <- true
	}()

	<-end
	logger.Info("server stopped")

}
