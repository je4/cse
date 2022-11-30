package main

import (
	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
	"path/filepath"
	"strings"
)

type Cfg_Google struct {
	Apikey           string `toml:"apikey"`
	CustomSearchKeys map[string]struct {
		Title string
		Link  string
		Key   string
		Name  string
	} `toml:"searchkeys"`
}

type CSEConfig struct {
	CertPem     string            `toml:"certpem"`
	KeyPem      string            `toml:"keypem"`
	LogFile     string            `toml:"logfile"`
	LogLevel    string            `toml:"loglevel"`
	LogFormat   string            `toml:"logformat"`
	AccessLog   string            `toml:"accesslog"`
	BaseDir     string            `toml:"basedir"`
	StaticDir   string            `toml:"staticdir"`
	TemplateDir string            `toml:"templatedir"`
	Addr        string            `toml:"addr"`
	AddrExt     string            `toml:"addrext"`
	User        string            `toml:"user"`
	Password    string            `toml:"password"`
	TemplateDev bool              `toml:"templatedev"`
	Domain      map[string]string `toml:"domain"`
	Google      Cfg_Google        `toml:"google"`
}

func LoadCSEConfig(fp string, conf *CSEConfig) error {
	_, err := toml.DecodeFile(fp, conf)
	if err != nil {
		return errors.Wrapf(err, "error loading config file %v", fp)
	}
	conf.BaseDir = strings.TrimRight(filepath.ToSlash(conf.BaseDir), "/")
	return nil
}
