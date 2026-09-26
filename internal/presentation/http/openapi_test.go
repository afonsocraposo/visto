package httpserver

import (
	"bufio"
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
	documented := map[string]bool{}
	path := ""
	for scanner := bufio.NewScanner(strings.NewReader(string(spec))); scanner.Scan(); {
		line := scanner.Text()
		if strings.HasPrefix(line, "  /") && strings.HasSuffix(line, ":") {
			path = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if strings.HasPrefix(line, "components:") {
			path = ""
		}
		if path != "" && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") {
			method := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(line), ":"))
			if method == "GET" || method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
				documented[method+" "+path] = true
			}
		}
	}
	routePattern := regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) (/api/v1/[^\"]+)"`)
	registered := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read HTTP source %s: %v", file, err)
		}
		for _, match := range routePattern.FindAllStringSubmatch(string(source), -1) {
			operation := match[1] + " " + strings.TrimPrefix(match[2], "/api/v1")
			registered[operation] = true
			if !documented[operation] {
				t.Errorf("OpenAPI does not document registered operation %s", operation)
			}
		}
	}
	for operation := range documented {
		if !registered[operation] {
			t.Errorf("OpenAPI documents unregistered operation %s", operation)
		}
	}
}
