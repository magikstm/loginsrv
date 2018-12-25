package caddy

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	. "github.com/stretchr/testify/assert"
	"github.com/tarent/loginsrv/login"
)

func TestSetup(t *testing.T) {
	for j, test := range []struct {
		input     string
		shouldErr bool
		config    login.Config
	}{
		{ //defaults
			input: `login {	
							simple bob=secret
							jwt-secret jwtsecret
							}`,
			shouldErr: false,
			config: login.Config{
				JwtSecret:              "jwtsecret",
				JwtAlgo:                "HS512",
				JwtExpiry:              24 * time.Hour,
				SuccessURL:             "/",
				Redirect:               true,
				RedirectQueryParameter: "backTo",
				RedirectCheckReferer:   true,
				LoginPath:              "/login",
				CookieName:             "jwt_token",
				CookieHTTPOnly:         true,
				Backends: login.Options{
					"simple": map[string]string{
						"bob": "secret",
					},
				},
				Oauth:       login.Options{},
				GracePeriod: 5 * time.Second,
			}},
		{
			input: `login {
							jwt-secret jwtsecret
							success_url successurl
							jwt_expiry 42h
							jwt_algo algo
							login_path /foo/bar
							redirect true
							redirect_query_parameter comingFrom
							redirect_check_referer true
							redirect_host_file domainWhitelist.txt
							cookie_name cookiename
							cookie_http_only false
							cookie_domain example.com
							cookie_expiry 23h23m
							simple bob=secret
							osiam endpoint=http://localhost:8080,client_id=example-client,client_secret=secret
							}`,
			shouldErr: false,
			config: login.Config{
				JwtSecret:              "jwtsecret",
				JwtAlgo:                "algo",
				JwtExpiry:              42 * time.Hour,
				SuccessURL:             "successurl",
				Redirect:               true,
				RedirectQueryParameter: "comingFrom",
				RedirectCheckReferer:   true,
				RedirectHostFile:       "domainWhitelist.txt",
				LoginPath:              "/foo/bar",
				CookieName:             "cookiename",
				CookieDomain:           "example.com",
				CookieExpiry:           23*time.Hour + 23*time.Minute,
				CookieHTTPOnly:         false,
				Backends: login.Options{
					"simple": map[string]string{
						"bob": "secret",
					},
					"osiam": map[string]string{
						"endpoint":      "http://localhost:8080",
						"client_id":     "example-client",
						"client_secret": "secret",
					},
				},
				Oauth:       login.Options{},
				GracePeriod: 5 * time.Second,
			}},
		{ // backwards compatibility
			// * login path as argument
			// * '-' in parameter names
			// * backend config by 'backend provider='
			input: `loginsrv /context {
                                        backend provider=simple,bob=secret
                                        cookie-name cookiename
										jwt-secret jwtsecret
                                }`,
			shouldErr: false,
			config: login.Config{
				JwtSecret:              "jwtsecret",
				JwtAlgo:                "HS512",
				JwtExpiry:              24 * time.Hour,
				SuccessURL:             "/",
				Redirect:               true,
				RedirectQueryParameter: "backTo",
				RedirectCheckReferer:   true,
				LoginPath:              "/context/login",
				CookieName:             "cookiename",
				CookieHTTPOnly:         true,
				Backends: login.Options{
					"simple": map[string]string{
						"bob": "secret",
					},
				},
				Oauth:       login.Options{},
				GracePeriod: 5 * time.Second,
			}},
		{ // backwards compatibility
			// * login path as argument
			// * '-' in parameter names
			// * backend config by 'backend provider='
			input: `loginsrv / {
                                        backend provider=simple,bob=secret
                                        cookie-name cookiename
										jwt-secret jwtsecret
                                }`,
			shouldErr: false,
			config: login.Config{
				JwtSecret:              "jwtsecret",
				JwtAlgo:                "HS512",
				JwtExpiry:              24 * time.Hour,
				SuccessURL:             "/",
				Redirect:               true,
				RedirectQueryParameter: "backTo",
				RedirectCheckReferer:   true,
				LoginPath:              "/login",
				CookieName:             "cookiename",
				CookieHTTPOnly:         true,
				Backends: login.Options{
					"simple": map[string]string{
						"bob": "secret",
					},
				},
				Oauth:       login.Options{},
				GracePeriod: 5 * time.Second,
			}},

		// error cases
		{ // duration parse error
			input: `login {
							simple bob=secret
							jwt-secret jwtsecret
							}`,
			shouldErr: false,
			config: login.Config{
				JwtSecret:              "jwtsecret",
				JwtAlgo:                "HS512",
				JwtExpiry:              24 * time.Hour,
				SuccessURL:             "/",
				Redirect:               true,
				RedirectQueryParameter: "backTo",
				RedirectCheckReferer:   true,
				LoginPath:              "/login",
				CookieName:             "jwt_token",
				CookieHTTPOnly:         true,
				Backends: login.Options{
					"simple": map[string]string{
						"bob": "secret",
					},
				},
				Oauth:       login.Options{},
				GracePeriod: 5 * time.Second,
			}},
		{input: "login {\n}", shouldErr: true},
		{input: "login xx yy {\n}", shouldErr: true},
		{input: "login {\n cookie_http_only 42d \n simple bob=secret \n}", shouldErr: true},
		{input: "login {\n unknown property \n simple bob=secret \n}", shouldErr: true},
		{input: "login {\n backend \n}", shouldErr: true},
		{input: "login {\n backend provider=foo\n}", shouldErr: true},
		{input: "login {\n backend kk\n}", shouldErr: true},
	} {
		t.Run(fmt.Sprintf("test %v", j), func(t *testing.T) {
			c := caddy.NewTestController("http", test.input)
			err := setup(c)
			if test.shouldErr {
				Error(t, err, "test ")
				return
			}
			NoError(t, err)
			mids := httpserver.GetConfig(c).Middleware()
			if len(mids) == 0 {
				t.Errorf("no middlewares created in test #%v", j)
				return
			}
			middleware := mids[len(mids)-1](nil).(*CaddyHandler)
			Equal(t, &test.config, middleware.config)
		})
	}
}

func TestSetup_RelativeFiles(t *testing.T) {
	caddyfile := `loginsrv {
                        template myTemplate.tpl
                        redirect_host_file redirectDomains.txt
                        simple bob=secret
                      }`
	root, _ := ioutil.TempDir("", "")

	c := caddy.NewTestController("http", caddyfile)
	c.Key = "RelativeTemplateFileTest"
	config := httpserver.GetConfig(c)
	config.Root = root

	err := setup(c)
	NoError(t, err)
	mids := httpserver.GetConfig(c).Middleware()
	if len(mids) == 0 {
		t.Errorf("no middlewares created")
	}
	middleware := mids[len(mids)-1](nil).(*CaddyHandler)

	Equal(t, filepath.FromSlash(root + "/myTemplate.tpl"), middleware.config.Template)
	Equal(t, "redirectDomains.txt", middleware.config.RedirectHostFile)
}
