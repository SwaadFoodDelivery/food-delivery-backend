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
	accessKey string
	secretKey string
	region    string
	endpoint  string
}

func NewDevProvider(accessKey, secretKey, region, endpoint string) *DevProvider {
	return &DevProvider{
		accessKey: strings.TrimSpace(accessKey),
		secretKey: strings.TrimSpace(secretKey),
		region:    strings.TrimSpace(region),
		endpoint:  strings.TrimSpace(endpoint),
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

	host, scheme, canonicalURI, err := d.hostAndURI(bucket, key)
	if err != nil {
		return nil, err
	}

	params := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    d.accessKey + "/" + credentialScope,
		"X-Amz-Date":          amzDate,
		"X-Amz-Expires":       fmt.Sprintf("%d", expires),
		"X-Amz-SignedHeaders": "host",
	}
	canonicalQuery := canonicalizeQuery(params)
	canonicalHeaders := "host:" + host + "\n"
	signedHeaders := "host"
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
		Headers:   map[string]string{"Content-Type": in.ContentType},
		ExpiresAt: now.Add(time.Duration(expires) * time.Second),
	}, nil
}

func (d *DevProvider) hostAndURI(bucket, key string) (host, scheme, canonicalURI string, err error) {
	safeKey := escapeS3Key(key)
	if d.endpoint == "" {
		host = fmt.Sprintf("%s.s3.%s.amazonaws.com", bucket, d.region)
		scheme = "https"
		canonicalURI = "/" + safeKey
		return
	}

	u, parseErr := url.Parse(d.endpoint)
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
