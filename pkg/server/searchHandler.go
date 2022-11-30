package server

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"google.golang.org/api/customsearch/v1"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type GoogleResultItem struct {
	Title                           template.HTML
	Snippet                         template.HTML
	Thumbnail                       string
	ThumbnailWidth, ThumbnailHeight int64
	Domain                          string
	Link                            string
	Mimetype                        string
	FileFormat                      string
}

type CSEThumbnail struct {
	Src    string `json:"src"`
	Width  string `json:"width"`
	Height string `json:"height"`
}
type ResultItemPagemap struct {
	Thumbnail []CSEThumbnail `json:"cse_thumbnail"`
}

func (s *Server) SearchHandler(w http.ResponseWriter, r *http.Request) {
	if s.username != "" {
		username, password, ok := r.BasicAuth()
		if !(ok && password == s.password && username == s.username) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Find Basel", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	vars := mux.Vars(r)
	search := strings.TrimSpace(vars["search"])
	if search == "" {
		http.Redirect(w, r, fmt.Sprintf(s.AddrExt), http.StatusTemporaryRedirect)
		return
	}
	resort, ok := vars["resort"]
	if !ok {
		http.Error(w, "invalid url", http.StatusNotFound)
		return
	}
	if s.templateDev {
		if err := s.InitTemplates(); err != nil {
			s.log.Errorf("error initializing templates: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("%v", err)))
			return
		}
	}
	tpl := s.templates["search.gohtml"]

	var err error
	var start int64

	startStr := r.URL.Query().Get("start")
	if startStr != "" {
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			s.log.Warningf("cannot parse start parameter %s", startStr)
			start = 0
		}
	}

	resort = strings.ToLower(resort)
	result := &struct {
		Resort            string
		Resorts           map[string]SearchResort
		Canonical         string
		SearchString      string
		NumResult         int64
		TotalResult       string
		SearchResultStart int64
		SearchResultRows  int64
		Items             []*GoogleResultItem
		ErrMessage        string
	}{
		Resort:            resort,
		Resorts:           s.resorts,
		Canonical:         fmt.Sprintf("%s/search/%s", s.AddrExt, resort),
		SearchString:      search,
		SearchResultStart: start,
		Items:             []*GoogleResultItem{},
	}

	res, ok := s.resorts[resort]
	if !ok {
		http.Error(w, fmt.Sprintf("search resort %s not found", resort), http.StatusNotFound)
		return

	}
	var cx = res.Key
	var resp *customsearch.Search

	cacheKey := fmt.Sprintf("%v-%s", start, search)
	cacheContent, err := s.cache.Get(cacheKey)
	if err != nil {
		s.log.Infof("cache miss for %s", cacheKey)
	} else {
		s.log.Infof("cache hit for %s", cacheKey)
		var ok bool
		resp, ok = cacheContent.(*customsearch.Search)
		if !ok {
			resp = nil
			s.log.Errorf("invalid type in cache")
		}
	}

	if resp == nil {
		s.log.Infof("querying %s", cacheKey)
		resp, err = s.search.Cse.List().Q(search).Start(start).Cx(cx).Do()
		if err != nil {
			result.ErrMessage = fmt.Sprintf("cannot search: %v", err)
			s.log.Errorf("cannot search: %v", err)
			if err := tpl.Execute(w, result); err != nil {
				s.log.Errorf("error executing search template: %v", err)
			}
			return
		}
		s.cache.Set(cacheKey, resp)
	}
	numResult, _ := strconv.ParseInt(resp.SearchInformation.TotalResults, 10, 64)

	result.NumResult = numResult
	result.TotalResult = resp.SearchInformation.FormattedTotalResults
	result.SearchResultRows = int64(len(resp.Items))

	for _, r := range resp.Items {
		item := &GoogleResultItem{
			Title:           template.HTML(r.HtmlTitle),
			Snippet:         template.HTML(r.HtmlSnippet),
			Thumbnail:       "",
			ThumbnailWidth:  0,
			ThumbnailHeight: 0,
			Domain:          r.DisplayLink,
			Link:            r.Link,
			Mimetype:        r.Mime,
			FileFormat:      r.FileFormat,
		}
		if r.Pagemap != nil {
			pagemap := &ResultItemPagemap{Thumbnail: []CSEThumbnail{}}
			if err := json.Unmarshal(r.Pagemap, pagemap); err != nil {
			} else {
				if len(pagemap.Thumbnail) > 0 {
					item.Thumbnail = pagemap.Thumbnail[0].Src
					widthStr := pagemap.Thumbnail[0].Width
					item.ThumbnailWidth, err = strconv.ParseInt(widthStr, 10, 64)
					heightStr := pagemap.Thumbnail[0].Height
					item.ThumbnailHeight, err = strconv.ParseInt(heightStr, 10, 64)
				}
			}
		}
		if r.Image != nil {
			item.Thumbnail = r.Image.ThumbnailLink
			item.ThumbnailWidth = r.Image.ThumbnailWidth
			item.ThumbnailHeight = r.Image.ThumbnailHeight
		}
		newDomain, ok := s.domain[item.Domain]
		if ok {
			item.Domain = newDomain
		}
		result.Items = append(result.Items, item)
	}

	if err := tpl.Execute(w, result); err != nil {
		s.log.Errorf("error executing search template: %v", err)
	}
}
