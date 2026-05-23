package tls

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// route53Client handles DNS record operations for ACME challenges.
type route53Client struct {
	accessKeyID string
	secretKey   string
	hostedZone  string
}

func (r *route53Client) upsertTXTRecord(ctx context.Context, name, value string) error {
	return r.changeRecord(ctx, "UPSERT", name, value)
}

func (r *route53Client) deleteTXTRecord(ctx context.Context, name, value string) error {
	return r.changeRecord(ctx, "DELETE", name, value)
}

func (r *route53Client) changeRecord(ctx context.Context, action, name, value string) error {
	// Ensure FQDN
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ChangeResourceRecordSetsRequest xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <ChangeBatch>
    <Changes>
      <Change>
        <Action>%s</Action>
        <ResourceRecordSet>
          <Name>%s</Name>
          <Type>TXT</Type>
          <TTL>60</TTL>
          <ResourceRecords>
            <ResourceRecord>
              <Value>"%s"</Value>
            </ResourceRecord>
          </ResourceRecords>
        </ResourceRecordSet>
      </Change>
    </Changes>
  </ChangeBatch>
</ChangeResourceRecordSetsRequest>`, action, name, value)

	url := fmt.Sprintf("https://route53.amazonaws.com/2013-04-01/hostedzone/%s/rrset", r.hostedZone)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")

	r.signRequest(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("route53 request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("route53 %s failed (status %d): %s", action, resp.StatusCode, string(respBody))
	}

	// Wait for change to propagate if upserting
	if action == "UPSERT" {
		respBody, _ := io.ReadAll(resp.Body)
		return r.waitForChange(ctx, respBody)
	}
	return nil
}

func (r *route53Client) waitForChange(ctx context.Context, responseBody []byte) error {
	var result struct {
		ChangeInfo struct {
			ID string `xml:"Id"`
		} `xml:"ChangeInfo"`
	}
	xml.Unmarshal(responseBody, &result)
	if result.ChangeInfo.ID == "" {
		return nil
	}

	// Poll until INSYNC (max 120s)
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		url := fmt.Sprintf("https://route53.amazonaws.com/2013-04-01%s", result.ChangeInfo.ID)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		r.signRequest(req)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if strings.Contains(string(body), "INSYNC") {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return nil // proceed anyway after timeout
}

// signRequest adds AWS Signature V4 headers (simplified for Route53).
func (r *route53Client) signRequest(req *http.Request) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.URL.Host)

	// Simplified SigV4 for Route53
	region := "us-east-1" // Route53 is always us-east-1
	service := "route53"

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)

	// Canonical request
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	bodyHash := sha256Hex(bodyBytes)

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n", req.URL.Host, amzDate)
	signedHeaders := "host;x-amz-date"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method, req.URL.Path, req.URL.RawQuery,
		canonicalHeaders, signedHeaders, bodyHash)

	// String to sign
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, credentialScope, sha256Hex([]byte(canonicalRequest)))

	// Signing key
	kDate := hmacSHA256([]byte("AWS4"+r.secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		r.accessKeyID, credentialScope, signedHeaders, signature))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
