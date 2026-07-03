package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

type DevProvider struct {
	accessKey      string
	secretKey      string
	region         string
	endpoint       string
	presignBaseURL string
}

func NewDevProvider(accessKey, secretKey, region, endpoint, presignBaseURL string) *DevProvider {
	return &DevProvider{
		accessKey:      strings.TrimSpace(accessKey),
		secretKey:      strings.TrimSpace(secretKey),
		region:         strings.TrimSpace(region),
		endpoint:       strings.TrimSpace(endpoint),
		presignBaseURL: strings.TrimSpace(presignBaseURL),
	}
}

func (d *DevProvider) PresignPut(_ context.Context, in PresignPutInput) (*PresignPutOutput, error) {
	if d.accessKey == "" || d.secretKey == "" || d.region == "" {
		return nil, fmt.Errorf("s3 dev provider is not configured")
	}
	bucket := strings.TrimSpace(in.Bucket)
	key := strings.TrimSpace(in.Key)
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("bucket and key are required")
	}

	expires := int(in.ExpiresIn.Seconds())
	if expires <= 0 {
		expires = 300
	}
	if expires > 604800 {
		expires = 604800
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	credentialScope := dateStamp + "/" + d.region + "/s3/aws4_request"

	signingEndpoint := d.endpoint
	if d.presignBaseURL != "" {
		signingEndpoint = d.presignBaseURL
	}
	host, scheme, canonicalURI, err := hostAndURI(signingEndpoint, d.region, bucket, key)
	if err != nil {
		return nil, err
	}

	signedHeaderNames := []string{"host"}
	canonicalHeaders := "host:" + host + "\n"
	outputHeaders := map[string]string{}
	if ct := strings.TrimSpace(in.ContentType); ct != "" {
		signedHeaderNames = append(signedHeaderNames, "content-type")
		sort.Strings(signedHeaderNames)
		// rebuild canonicalHeaders in sorted order
		canonicalHeaders = "content-type:" + ct + "\n" + "host:" + host + "\n"
		outputHeaders["Content-Type"] = ct
	}
	signedHeaders := strings.Join(signedHeaderNames, ";")

	params := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    d.accessKey + "/" + credentialScope,
		"X-Amz-Date":          amzDate,
		"X-Amz-Expires":       fmt.Sprintf("%d", expires),
		"X-Amz-SignedHeaders": signedHeaders,
	}
	canonicalQuery := canonicalizeQuery(params)
	payloadHash := "UNSIGNED-PAYLOAD"
	canonicalRequest := strings.Join([]string{
		"PUT",
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	hashedCanonicalRequest := sha256Hex(canonicalRequest)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hashedCanonicalRequest,
	}, "\n")

	signingKey := awsV4SigningKey(d.secretKey, dateStamp, d.region, "s3")
	signature := hmacHex(signingKey, stringToSign)

	signedValues := url.Values{}
	for k, v := range params {
		signedValues.Set(k, v)
	}
	signedValues.Set("X-Amz-Signature", signature)

	presignedURL := fmt.Sprintf("%s://%s%s?%s", scheme, host, canonicalURI, signedValues.Encode())

	return &PresignPutOutput{
		URL:       presignedURL,
		Method:    "PUT",
		Headers:   outputHeaders,
		ExpiresAt: now.Add(time.Duration(expires) * time.Second),
	}, nil
}

func hostAndURI(endpoint, region, bucket, key string) (host, scheme, canonicalURI string, err error) {
	safeKey := escapeS3Key(key)
	if endpoint == "" {
		host = fmt.Sprintf("%s.s3.%s.amazonaws.com", bucket, region)
		scheme = "https"
		canonicalURI = "/" + safeKey
		return
	}

	u, parseErr := url.Parse(endpoint)
	if parseErr != nil {
		err = fmt.Errorf("invalid S3 endpoint: %w", parseErr)
		return
	}
	host = u.Host
	scheme = u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	basePath := strings.TrimSuffix(u.Path, "/")
	canonicalURI = path.Clean(basePath + "/" + bucket + "/" + safeKey)
	if !strings.HasPrefix(canonicalURI, "/") {
		canonicalURI = "/" + canonicalURI
	}
	return
}

func escapeS3Key(key string) string {
	parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(key, "/")), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func canonicalizeQuery(q map[string]string) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	vals := make([]string, 0, len(q))
	for _, k := range keys {
		vals = append(vals, url.QueryEscape(k)+"="+url.QueryEscape(q[k]))
	}
	return strings.Join(vals, "&")
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func hmacBytes(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

func hmacHex(key []byte, data string) string {
	return hex.EncodeToString(hmacBytes(key, data))
}

func awsV4SigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacBytes([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacBytes(kDate, region)
	kService := hmacBytes(kRegion, service)
	return hmacBytes(kService, "aws4_request")
}
