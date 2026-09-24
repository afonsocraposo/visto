package httpserver

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestOpenAPI_GivenRegisteredRESTRoutes_WhenChecked_ThenEachPublicRouteIsDocumented(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(sourceFile), "*.go"))
	if err != nil {
		t.Fatalf("list HTTP source files: %v", err)
	}
	spec, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "../../../api/openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}
	routePattern := regexp.MustCompile(`mux\.HandleFunc\("(?:GET|POST|PUT|PATCH|DELETE) (/api/v1/[^\"]+)"`)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read HTTP source %s: %v", file, err)
		}
		for _, match := range routePattern.FindAllStringSubmatch(string(source), -1) {
			path := strings.TrimPrefix(match[1], "/api/v1")
			if !strings.Contains(string(spec), "\n  "+path+":") {
				t.Errorf("OpenAPI does not document registered route %s", match[1])
			}
		}
	}
}
