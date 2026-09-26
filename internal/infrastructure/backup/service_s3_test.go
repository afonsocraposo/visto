package backup

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

type testCipher struct{}

func (testCipher) Encrypt(s string) (string, error) { return "encrypted:" + s, nil }
func (testCipher) Decrypt(s string) (string, error) { return strings.TrimPrefix(s, "encrypted:"), nil }
func TestBackupSettingsEncryptSecretAndKeepItOnUpdate(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := &Service{Store: store, Cipher: testCipher{}, DefaultInterval: 24 * time.Hour}
	b, _ := service.Settings(ctx)
	b.Destination = "s3"
	b.Bucket = "bucket"
	b.Region = "us-east-1"
	b.AccessKeyID = "key"
	if err := service.Save(ctx, b, "secret"); err != nil {
		t.Fatal(err)
	}
	saved, _ := service.Settings(ctx)
	if saved.SecretCiphertext != "encrypted:secret" || !saved.HasSecret {
		t.Fatalf("incorrect secret state: %+v", saved)
	}
	saved.IntervalSeconds = 3600
	if err := service.Save(ctx, saved, ""); err != nil {
		t.Fatal(err)
	}
	saved, _ = service.Settings(ctx)
	if saved.SecretCiphertext != "encrypted:secret" {
		t.Fatal("replacement erased saved secret")
	}
}
func TestS3UploadAndPrefixRetention(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		switch r.Method {
		case http.MethodPut:
			body := make([]byte, r.ContentLength)
			r.Body.Read(body)
			objects[key] = body
			w.WriteHeader(200)
		case http.MethodDelete:
			delete(objects, key)
			w.WriteHeader(204)
		case http.MethodGet:
			type item struct {
				Key string `xml:"Key"`
			}
			type listing struct {
				XMLName     xml.Name `xml:"ListBucketResult"`
				Contents    []item   `xml:"Contents"`
				IsTruncated bool     `xml:"IsTruncated"`
			}
			result := listing{}
			for k := range objects {
				if strings.HasPrefix(k, r.URL.Query().Get("prefix")) {
					result.Contents = append(result.Contents, item{Key: k})
				}
			}
			xml.NewEncoder(w).Encode(result)
		}
	})
	cfg := S3Config{Bucket: "bucket", Region: "us-east-1", Endpoint: "https://s3.test", AccessKeyID: "id", SecretKey: "secret", PathStyle: true, Prefix: "visto/", MaxKeep: 2}
	client := S3Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Result(), nil
	})}}
	file := filepath.Join(t.TempDir(), "backup.db")
	db, err := sql.Open("sqlite", file)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("CREATE TABLE data(value TEXT)")
	db.Close()
	if err := client.Test(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("visto/visto-backup-2026010%dT000000.000000000Z.db", i)
		if err := client.Upload(context.Background(), cfg, key, file); err != nil {
			t.Fatal(err)
		}
	}
	objects["other/object"] = []byte("keep")
	objects["visto/visto-manual-20260101T000000.000000000Z.db"] = []byte("keep")
	if err := client.Prune(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if len(objects) != 4 {
		t.Fatalf("unexpected object count: %d", len(objects))
	}
	if _, ok := objects["visto/visto-backup-20260101T000000.000000000Z.db"]; ok {
		t.Fatal("old scheduled backup was retained")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestS3OnlyBackupCanBeRestoredAndRemovesStagingFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	source := filepath.Join(root, "source.db")
	store, err := sqlite.Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB.Exec(`CREATE TABLE backup_smoke(value TEXT); INSERT INTO backup_smoke VALUES('saved')`); err != nil {
		t.Fatal(err)
	}
	objects := map[string][]byte{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			objects[key] = body
			w.WriteHeader(200)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(`<ListBucketResult><IsTruncated>false</IsTruncated></ListBucketResult>`))
		case http.MethodDelete:
			delete(objects, key)
			w.WriteHeader(204)
		}
	})
	client := S3Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Result(), nil
	})}}
	service := &Service{Store: store, Cipher: testCipher{}, DatabasePath: source, Directory: filepath.Join(root, "local"), DefaultInterval: 24 * time.Hour, LocalRetention: 30 * 24 * time.Hour, S3: client}
	settings, _ := service.Settings(ctx)
	settings.Destination = "s3"
	settings.Bucket = "bucket"
	settings.Region = "us-east-1"
	settings.Endpoint = "https://s3.test"
	settings.PathStyle = true
	settings.AccessKeyID = "id"
	if err := service.Save(ctx, settings, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := service.RunOnce(ctx, false); err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 {
		t.Fatalf("uploaded objects = %d", len(objects))
	}
	var snapshot []byte
	for _, value := range objects {
		snapshot = value
	}
	restored := filepath.Join(root, "restored.db")
	if err := os.WriteFile(restored, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(`SELECT value FROM backup_smoke`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "saved" {
		t.Fatalf("restored value = %q", value)
	}
	matches, err := filepath.Glob(filepath.Join(root, "visto-s3-backup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary backup left behind: %v", matches)
	}
}
func TestS3OnlyUploadFailureRemovesStagingFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	source := filepath.Join(root, "source.db")
	store, err := sqlite.Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	client := S3Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Result(), nil
	})}}
	service := &Service{Store: store, Cipher: testCipher{}, DatabasePath: source, Directory: filepath.Join(root, "local"), DefaultInterval: 24 * time.Hour, LocalRetention: 30 * 24 * time.Hour, S3: client}
	settings, _ := service.Settings(ctx)
	settings.Destination = "s3"
	settings.Bucket = "bucket"
	settings.Region = "us-east-1"
	settings.Endpoint = "https://s3.test"
	settings.PathStyle = true
	settings.AccessKeyID = "id"
	if err := service.Save(ctx, settings, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := service.RunOnce(ctx, false); err == nil {
		t.Fatal("upload failure was ignored")
	}
	matches, err := filepath.Glob(filepath.Join(root, "visto-s3-backup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary backup left behind: %v", matches)
	}
	saved, _ := service.Settings(ctx)
	if saved.LastError == "" {
		t.Fatal("failure status was not saved")
	}
}
