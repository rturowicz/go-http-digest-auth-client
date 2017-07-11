package digest_auth_client

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

type DigestRequest struct {
	Body     string
	Method   string
	Password string
	Uri      string
	Username string
	Auth     *authorization
	Wa       *wwwAuthenticate
	ContentType	string
}

type DigestTransport struct {
	Password string
	Username string
}

func NewRequest(username string, password string, method string, uri string, body string, content_type string) DigestRequest {
	dr := DigestRequest{}
	dr.UpdateRequest(username, password, method, uri, body, content_type)
	return dr
}

func NewTransport(username string, password string) DigestTransport {
	dt := DigestTransport{}
	dt.Password = password
	dt.Username = username
	return dt
}

func (dr *DigestRequest) UpdateRequest(username string,
	password string, method string, uri string, body string, content_type string) *DigestRequest {

	dr.Body = body
	dr.Method = method
	dr.Password = password
	dr.Uri = uri
	dr.Username = username
	dr.ContentType = content_type
	return dr
}

func (dt *DigestTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	username := dt.Username
	password := dt.Password
	method := req.Method
	uri := req.URL.String()

	var body string
	if req.Body != nil {
		buf := new(bytes.Buffer)
		buf.ReadFrom(req.Body)
		body = buf.String()
	}

	dr := NewRequest(username, password, method, uri, body, req.Header.Get("Content-Type"))
	return dr.Execute()
}

func (dr *DigestRequest) Execute() (resp *http.Response, err error) {

	if dr.Auth == nil {
		var req *http.Request
		if req, err = http.NewRequest(dr.Method, dr.Uri, bytes.NewReader([]byte(dr.Body))); err != nil {
			return nil, err
		}

		if dr.ContentType != "" {
			req.Header.Set("Content-Type", dr.ContentType)
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		resp, err = client.Do(req)

		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 401 {
			return dr.executeNewDigest(resp)
		}
		return
	}

	return dr.executeExistingDigest()
}

func (dr *DigestRequest) executeNewDigest(resp *http.Response) (*http.Response, error) {
	var (
		auth *authorization
		err  error
		wa   *wwwAuthenticate
	)

	waString := resp.Header.Get("WWW-Authenticate")
	if waString == "" {
		return nil, fmt.Errorf("Failed to get WWW-Authenticate header, please check your server configuration.")
	}
	wa = newWwwAuthenticate(waString)
	dr.Wa = wa

	if auth, err = newAuthorization(dr); err != nil {
		return nil, err
	}
	authString := auth.toString()

	if resp, err := dr.executeRequest(authString); err != nil {
		return nil, err
	} else {
		dr.Auth = auth
		return resp, nil
	}
}

func (dr *DigestRequest) executeExistingDigest() (*http.Response, error) {
	var (
		auth *authorization
		err  error
	)

	if auth, err = dr.Auth.refreshAuthorization(dr); err != nil {
		return nil, err
	}
	dr.Auth = auth

	authString := dr.Auth.toString()
	return dr.executeRequest(authString)
}

func (dr *DigestRequest) executeRequest(authString string) (*http.Response, error) {
	var (
		err error
		req *http.Request
	)

	if req, err = http.NewRequest(dr.Method, dr.Uri, bytes.NewReader([]byte(dr.Body))); err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", authString)

	if dr.ContentType != "" {
		req.Header.Set("Content-Type", dr.ContentType)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return client.Do(req)
}
