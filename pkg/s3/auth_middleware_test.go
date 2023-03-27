package s3

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/minio/minio-go/v7/pkg/signer"
)

func generateRandomBody(method string) io.Reader {
	if method == "GET" || method == "DELETE" {
		return nil
	}

	srand := rand.New(rand.NewSource(time.Now().UnixNano()))

	size := srand.Intn(2000) + 2000
	buf := make([]byte, size)
	srand.Read(buf)

	return bytes.NewReader(buf)
}

func TestAuthMiddlewareSanitizesInput(t *testing.T) {
	g := NewGomegaWithT(t)

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Add("Authorization", "AWS4-HMAC-SHA256 1=AccessKeyID/Time/Location/Service/Request, 2=2, 3=3")
	req.Header.Add("X-Amz-Date", "<script>foobar</script>")
	g.Expect(err).NotTo(HaveOccurred(), "failed creating test request")

	rr := httptest.NewRecorder()

	dummyHandler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {})
	authMW := AuthMiddleware("", "", dummyHandler)

	authMW.ServeHTTP(rr, req)

	g.Expect(rr.Code).To(Equal(http.StatusUnauthorized), "unexpected status code")
	respBody, err := io.ReadAll(rr.Body)
	g.Expect(err).NotTo(HaveOccurred(), "failed reading response body")

	g.Expect(string(respBody)).NotTo(ContainSubstring("<script>"))
	g.Expect(string(respBody)).To(ContainSubstring("&lt;script&gt;"))
}

func TestVerifySignature(t *testing.T) {
	var testcases []struct {
		name     string
		req      *http.Request
		expected error
	}

	g := NewGomegaWithT(t)

	accessKey, err := GenerateAccessKey(DefaultRandIntFunc)
	g.Expect(err).NotTo(HaveOccurred(), "failed generating access key")

	secretKey, err := GenerateSecretKey(DefaultRandIntFunc)
	g.Expect(err).NotTo(HaveOccurred(), "failed generating secret key")

	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		for _, region := range []string{"us-east-1", "us-west-1"} {
			for _, host := range []string{"", "localhost", "localhost:8080", "localhost:9000"} {
				for _, bucket := range []string{"", "testbucket"} {
					for _, object := range []string{"", "testobject"} {
						for _, query := range []string{"", "query", "a=b&c=d"} {
							// good case
							testcases = append(testcases, struct {
								name     string
								req      *http.Request
								expected error
							}{
								name: fmt.Sprintf("good - %s %s/%s/%s/%s?%s", method, region, host, bucket, object, query),
								req: func() *http.Request {
									req, err := http.NewRequest(method, fmt.Sprintf("http://%s/%s/%s?%s", host, bucket, object, query), generateRandomBody(method))
									if err != nil {
										t.Fatal(err)
									}

									signed := signer.SignV4(*req, string(accessKey), string(secretKey), "", region)
									return signed
								}(),
								expected: nil,
							})

							// bad signature cases
							testcases = append(testcases, struct {
								name     string
								req      *http.Request
								expected error
							}{
								name: fmt.Sprintf("bad signature - %s %s/%s/%s/%s?%s", method, region, host, bucket, object, query),
								req: func() *http.Request {
									req, err := http.NewRequest(method, fmt.Sprintf("http://%s/%s/%s?%s", host, bucket, object, query), generateRandomBody(method))
									if err != nil {
										t.Fatal(err)
									}
									signed := signer.SignV4(*req, string(accessKey), "invalid", "", region)
									return signed
								}(),
								expected: fmt.Errorf("access denied: signature does not match"),
							})

							// bad credential cases
							testcases = append(testcases, struct {
								name     string
								req      *http.Request
								expected error
							}{
								name: fmt.Sprintf("bad credential - %s %s/%s/%s/%s?%s", method, region, host, bucket, object, query),
								req: func() *http.Request {
									req, err := http.NewRequest(method, fmt.Sprintf("http://%s/%s/%s?%s", host, bucket, object, query), generateRandomBody(method))
									if err != nil {
										t.Fatal(err)
									}
									signed := signer.SignV4(*req, "invalid", string(secretKey), "", region)
									return signed
								}(),
								expected: fmt.Errorf("access denied: credential does not match"),
							})
						}
					}
				}
			}
		}
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			if tc.expected == nil {
				g.Expect(verifySignature(*tc.req, string(accessKey), string(secretKey))).To(Succeed())
			} else {
				g.Expect(verifySignature(*tc.req, string(accessKey), string(secretKey)).Error()).To(Equal(tc.expected.Error()))
			}
		})
	}
}
