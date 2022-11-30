package server

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
)

func (s *Server) IndexHandler(w http.ResponseWriter, req *http.Request) {
	if s.username != "" {
		username, password, ok := req.BasicAuth()
		if !(ok && password == s.password && username == s.username) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Find Basel", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}
	if s.templateDev {
		if err := s.InitTemplates(); err != nil {
			s.log.Errorf("error initializing templates: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("%v", err)))
			return
		}
	}

	vars := mux.Vars(req)
	resort, ok := vars["resort"]
	if !ok {
		http.Error(w, "invalid url", http.StatusNotFound)
		return
	}
	resort = strings.ToLower(resort)
	data := struct {
		Resort    string
		Resorts   map[string]SearchResort
		Canonical string
	}{
		Resort:    resort,
		Resorts:   s.resorts,
		Canonical: fmt.Sprintf("%s/search/%s", s.AddrExt, resort),
	}

	tpl := s.templates["index.gohtml"]
	if err := tpl.Execute(w, data); err != nil {
		s.log.Errorf("error executing search template: %v", err)
	}
}
