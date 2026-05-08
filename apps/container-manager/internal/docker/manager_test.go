package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistryHost(t *testing.T) {
	cases := map[string]string{
		"us-central1-docker.pkg.dev/proj/repo/img:tag":      "us-central1-docker.pkg.dev",
		"gcr.io/proj/img:tag":                                "gcr.io",
		"docker.io/library/alpine:latest":                    "docker.io",
		"alpine:latest":                                      "docker.io",
		"library/alpine":                                     "docker.io",
		"localhost:5000/foo/bar:latest":                      "localhost:5000",
		"mirror.gcr.io/library/postgres:18-alpine":           "mirror.gcr.io",
	}
	for in, want := range cases {
		assert.Equal(t, want, registryHost(in), "image=%q", in)
	}
}

func TestNeedsGoogleAuth(t *testing.T) {
	yes := []string{
		"us-central1-docker.pkg.dev/proj/repo/img:tag",
		"asia-northeast1-docker.pkg.dev/proj/repo/img:tag",
	}
	no := []string{
		"docker.io/library/alpine",
		"alpine:latest",
		"mirror.gcr.io/library/alpine",
		"gcr.io/proj/img:tag",
		"localhost:5000/img",
	}
	for _, img := range yes {
		assert.True(t, needsGoogleAuth(img), "expected google auth for %q", img)
	}
	for _, img := range no {
		assert.False(t, needsGoogleAuth(img), "expected no google auth for %q", img)
	}
}

type stubToken struct{ tok string }

func (s stubToken) Token(_ context.Context) (string, error) { return s.tok, nil }

func TestGoogleRegistryAuth_Encoding(t *testing.T) {
	m := &Manager{token: stubToken{tok: "ya29.test"}}
	auth, err := m.googleRegistryAuth(context.Background(), "us-central1-docker.pkg.dev")
	assert.NoError(t, err)

	raw, err := base64.URLEncoding.DecodeString(auth)
	assert.NoError(t, err)

	var got struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		ServerAddress string `json:"serveraddress"`
	}
	assert.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "oauth2accesstoken", got.Username)
	assert.Equal(t, "ya29.test", got.Password)
	assert.Equal(t, "us-central1-docker.pkg.dev", got.ServerAddress)
}
