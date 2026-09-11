package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var verificationProviders = []struct {
	name string
	new  func(string) Provider
}{
	{"dev", func(endpoint string) Provider {
		return NewDevProvider("test-access", "test-secret", "us-east-1", endpoint, "http://public.invalid")
	}},
	{"mock", func(endpoint string) Provider { return NewMockProvider(endpoint) }},
}

func TestObjectExistsHTTP(t *testing.T) {
	for _, provider := range verificationProviders {
		t.Run(provider.name, func(t *testing.T) {
			for _, tc := range []struct {
				name    string
				status  int
				length  string
				want    bool
				wantErr bool
			}{
				{"present", http.StatusOK, "42", true, false},
				{"not_found", http.StatusNotFound, "42", false, false},
				{"unauthorized", http.StatusUnauthorized, "42", false, true},
				{"forbidden", http.StatusForbidden, "42", false, true},
				{"server_error", http.StatusInternalServerError, "42", false, true},
				{"empty", http.StatusOK, "0", false, false},
				{"unknown_length", http.StatusOK, "", false, false},
				{"no_content", http.StatusNoContent, "", false, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if r.Method != http.MethodHead || r.URL.Path != "/storage/documents/user/file.pdf" {
							t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						}
						if provider.name == "dev" {
							verifySignature(t, r, "host", "60")
						}
						if tc.length != "" {
							w.Header().Set("Content-Length", tc.length)
						}
						w.WriteHeader(tc.status)
					}))
					defer server.Close()
					exists, err := provider.new(server.URL+"/storage").ObjectExists(context.Background(), "documents", "user/file.pdf")
					if exists != tc.want || (err != nil) != tc.wantErr {
						t.Fatalf("ObjectExists() = (%v, %v), want (%v, error=%v)", exists, err, tc.want, tc.wantErr)
					}
					if tc.wantErr && !strings.Contains(err.Error(), strconv.Itoa(tc.status)) {
						t.Errorf("error does not identify HTTP status: %v", err)
					}
					if requests.Load() != 1 {
						t.Fatalf("got %d HEAD requests, want 1", requests.Load())
					}
				})
			}
		})
	}
}

func TestObjectExistsDoesNotFollowRedirects(t *testing.T) {
	for _, provider := range verificationProviders {
		t.Run(provider.name, func(t *testing.T) {
			var followed atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				followed.Add(1)
				w.Header().Set("Content-Length", "42")
			}))
			defer target.Close()
			for _, status := range []int{301, 302, 303, 307, 308} {
				t.Run(strconv.Itoa(status), func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Location", target.URL)
						w.WriteHeader(status)
					}))
					defer server.Close()
					exists, err := provider.new(server.URL).ObjectExists(context.Background(), "bucket", "key")
					if exists || err == nil {
						t.Fatalf("redirect accepted: (%v, %v)", exists, err)
					}
				})
			}
			if followed.Load() != 0 {
				t.Fatalf("followed %d redirects", followed.Load())
			}
		})
	}
}

func TestObjectExistsNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	for _, provider := range verificationProviders {
		t.Run(provider.name, func(t *testing.T) {
			exists, err := provider.new(server.URL).ObjectExists(context.Background(), "bucket", "key")
			if exists || err == nil {
				t.Fatalf("network failure accepted: (%v, %v)", exists, err)
			}
		})
	}
}

func TestObjectExistsCancellationAndTimeout(t *testing.T) {
	for _, provider := range verificationProviders {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
			for _, mode := range []string{"already_canceled", "in_flight_cancel", "deadline", "client_timeout"} {
				t.Run(mode, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					wantErr := context.Canceled
					if mode == "already_canceled" {
						cancel()
					} else if mode == "deadline" {
						var deadlineCancel context.CancelFunc
						ctx, deadlineCancel = context.WithTimeout(ctx, 50*time.Millisecond)
						defer deadlineCancel()
						wantErr = context.DeadlineExceeded
					} else if mode == "client_timeout" {
						wantErr = context.DeadlineExceeded
					}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if mode == "in_flight_cancel" {
							cancel()
						}
						select {
						case <-r.Context().Done():
						case <-time.After(2 * objectVerificationTimeout):
						}
					}))
					defer server.Close()
					start := time.Now()
					exists, err := provider.new(server.URL).ObjectExists(ctx, "bucket", "key")
					if exists || !errors.Is(err, wantErr) {
						t.Fatalf("ObjectExists() = (%v, %v), want false, %v", exists, err, wantErr)
					}
					if elapsed := time.Since(start); elapsed > objectVerificationTimeout+time.Second {
						t.Errorf("verification took too long: %s", elapsed)
					}
				})
			}
		})
	}
}

func TestObjectExistsInvalidInput(t *testing.T) {
	for _, provider := range verificationProviders {
		t.Run(provider.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
			defer server.Close()
			for _, input := range [][2]string{{"", "key"}, {"bucket", " "}} {
				exists, err := provider.new(server.URL).ObjectExists(context.Background(), input[0], input[1])
				if exists || err == nil {
					t.Errorf("invalid input accepted: (%v, %v)", exists, err)
				}
			}
			if requests.Load() != 0 {
				t.Errorf("invalid input made %d requests", requests.Load())
			}
			for _, endpoint := range []string{"http://%zz", "ftp://example.invalid"} {
				exists, err := provider.new(endpoint).ObjectExists(context.Background(), "bucket", "key")
				if exists || err == nil {
					t.Errorf("invalid endpoint accepted: (%v, %v)", exists, err)
				}
			}
		})
	}
	exists, err := NewDevProvider("", "", "", "", "").ObjectExists(context.Background(), "bucket", "key")
	if exists || err == nil {
		t.Fatalf("missing configuration accepted: (%v, %v)", exists, err)
	}
}

func TestDevObjectExistsInternalEndpointAndEscaping(t *testing.T) {
	var publicRequests atomic.Int32
	public := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { publicRequests.Add(1) }))
	defer public.Close()
	key := "folder//../a b+c%?#é.pdf"
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/s3 base/documents/"+key {
			t.Errorf("unexpected object request: %s %s", r.Method, r.URL.Path)
		}
		if want := "/s3%20base/documents/folder//../a%20b%2Bc%25%3F%23%C3%A9.pdf"; r.URL.EscapedPath() != want {
			t.Errorf("escaped path = %q, want %q", r.URL.EscapedPath(), want)
		}
		verifySignature(t, r, "host", "60")
		w.Header().Set("Content-Length", "1")
	}))
	defer internal.Close()
	p := NewDevProvider("test-access", "test-secret", "us-east-1", internal.URL+"/s3%20base", public.URL)
	exists, err := p.ObjectExists(context.Background(), "documents", key)
	if !exists || err != nil {
		t.Fatalf("ObjectExists() = (%v, %v)", exists, err)
	}
	if publicRequests.Load() != 0 {
		t.Errorf("used public endpoint %d times", publicRequests.Load())
	}
}

// Recompute SigV4 from the received request without using production signing helpers.
func verifySignature(t *testing.T, r *http.Request, signedHeaders, expires string) {
	t.Helper()
	q := r.URL.Query()
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || q.Get("X-Amz-SignedHeaders") != signedHeaders || q.Get("X-Amz-Expires") != expires {
		t.Errorf("unexpected signing parameters: %v", q)
	}
	date, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	if err != nil {
		t.Errorf("invalid signing date: %v", err)
		return
	}
	scope := date.Format("20060102") + "/us-east-1/s3/aws4_request"
	if q.Get("X-Amz-Credential") != "test-access/"+scope {
		t.Errorf("unexpected credential scope: %s", q.Get("X-Amz-Credential"))
	}
	signature := q.Get("X-Amz-Signature")
	q.Del("X-Amz-Signature")
	headers := ""
	for _, name := range strings.Split(signedHeaders, ";") {
		value := r.Header.Get(name)
		if name == "host" {
			value = r.Host
		}
		headers += name + ":" + value + "\n"
	}
	canonical := strings.Join([]string{r.Method, r.URL.EscapedPath(), strings.ReplaceAll(q.Encode(), "+", "%20"), headers, signedHeaders, "UNSIGNED-PAYLOAD"}, "\n")
	hash := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + q.Get("X-Amz-Date") + "\n" + scope + "\n" + hex.EncodeToString(hash[:])
	key := []byte("AWS4test-secret")
	for _, data := range []string{date.Format("20060102"), "us-east-1", "s3", "aws4_request", toSign} {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(data))
		key = mac.Sum(nil)
	}
	if want := hex.EncodeToString(key); signature != want {
		t.Errorf("invalid %s signature: got %s, want %s", r.Method, signature, want)
	}
}

func TestPresignPutRegression(t *testing.T) {
	for _, contentType := range []string{"", "application/octet-stream"} {
		for _, ttl := range []struct {
			input time.Duration
			want  string
		}{{0, "300"}, {time.Minute, "60"}, {8 * 24 * time.Hour, "604800"}} {
			t.Run(contentType+"/"+ttl.want, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPut || r.URL.Path != "/public/bucket/folder/file name.pdf" {
						t.Errorf("unexpected PUT request: %s %s", r.Method, r.URL.Path)
					}
					signedHeaders := "host"
					if contentType != "" {
						signedHeaders = "content-type;host"
					}
					verifySignature(t, r, signedHeaders, ttl.want)
				}))
				defer server.Close()
				p := NewDevProvider("test-access", "test-secret", "us-east-1", "http://internal.invalid", server.URL+"/public")
				out, err := p.PresignPut(context.Background(), PresignPutInput{Bucket: "bucket", Key: "folder/file name.pdf", ContentType: contentType, ExpiresIn: ttl.input})
				if err != nil {
					t.Fatal(err)
				}
				if out.Method != http.MethodPut || out.Headers["Content-Type"] != contentType {
					t.Fatalf("unexpected PUT output: %+v", out)
				}
				req, err := http.NewRequest(out.Method, out.URL, nil)
				if err != nil {
					t.Fatal(err)
				}
				for name, value := range out.Headers {
					req.Header.Set(name, value)
				}
				resp, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				seconds, _ := strconv.Atoi(ttl.want)
				date, err := time.Parse("20060102T150405Z", req.URL.Query().Get("X-Amz-Date"))
				if err != nil {
					t.Fatal(err)
				}
				if !out.ExpiresAt.Truncate(time.Second).Equal(date.Add(time.Duration(seconds) * time.Second)) {
					t.Errorf("unexpected expiry: %s", out.ExpiresAt)
				}
			})
		}
	}
}
