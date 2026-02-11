package sf_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

func expectURL(url string, code int, contentMatch string) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	Eventually(func() error {
		resp, err := client.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != code {
			return fmt.Errorf("status code mismatch: got %d, want %d", resp.StatusCode, code)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}

		if !strings.Contains(string(body), contentMatch) {
			// truncate body if too long for error message
			bodyStr := string(body)
			if len(bodyStr) > 200 {
				bodyStr = bodyStr[:200] + "..."
			}
			return fmt.Errorf("content mismatch: '%s' not found in body: %s", contentMatch, bodyStr)
		}

		return nil
	}, 5*time.Minute, 5*time.Second).Should(Succeed(), "URL: %s", url)
}

func testBasicURL(cr sfv1.SoftwareFactory) {
	fqdn := cr.Spec.FQDN

	expectURL(fmt.Sprintf("https://gerrit.%s", fqdn), 200, "PolyGerrit")
	expectURL(fmt.Sprintf("https://%s/zuul/api/info", fqdn), 200, "info")
	expectURL(fmt.Sprintf("https://%s/zuul/status", fqdn), 200, "Zuul")
	expectURL(fmt.Sprintf("https://%s/logs/", fqdn), 200, "Index of /logs")
	expectURL(fmt.Sprintf("https://%s/nodepool/api/ready", fqdn), 200, "OK")
	expectURL(fmt.Sprintf("https://%s/codesearch/", fqdn), 200, "open_search.xml")
}
