package server

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Server struct {
	mux *http.ServeMux
}

func New(options ...func(*Server)) *Server {
	s := &Server{mux: http.NewServeMux()}

	for _, f := range options {
		f(s)
	}

	s.mux.HandleFunc("/", s.index)
	s.mux.HandleFunc("/healthz/", s.healthz)

	return s
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	runtime := map[string]string{
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"version":       runtime.Version(),
		"max_procs":     strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"num_goroutine": strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"num_cpu":       strconv.FormatInt(int64(runtime.NumCPU()), 10),
	}
	runtime["hostname"], _ = os.Hostname()

	labels, err := filesToMap("/etc/podinfod/metadata/labels")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	annotations, err := filesToMap("/etc/podinfod/metadata/annotations")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	resp := &Response{
		Environment: os.Environ(),
		Runtime:     runtime,
		Labels:      labels,
		Annotations: annotations,
	}

	d, err := yaml.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	w.Header().Set("Content-Type", "text/x-yaml")
	w.Write(d)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", runtime.Version())

	s.mux.ServeHTTP(w, r)
}

func filesToMap(dir string) (map[string]string, error) {
	files := make([]string, 0)
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		files = append(files, path)

		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Reading from %v failed", dir)
	}
	list := make(map[string]string, 0)
	for _, path := range files {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrapf(err, "Reading %v failed", path)
		}
		content := strings.TrimPrefix(string(data), "\n")
		kv := strings.Split(content, "=")
		if len(kv) > 1 {
			list[kv[0]] = strings.TrimPrefix(kv[1], "\n")
		} else {
			list[kv[0]] = ""
		}
	}
	return list, nil
}