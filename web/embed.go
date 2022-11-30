package web

import "embed"

//go:embed template/search.gohtml
//go:embed template/index.gohtml
var TemplateFS embed.FS

//go:embed static/*
var StaticFS embed.FS
