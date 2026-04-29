package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// CCTunnelTokenResolver resolves a username by calling cc-tunnel's /app-auth/me endpoint.
type CCTunnelTokenResolver struct {
	ccTunnelURL string
	httpClient  *http.Client
}

func NewCCTunnelTokenResolver(ccTunnelURL string) *CCTunnelTokenResolver {
	return &CCTunnelTokenResolver{
		ccTunnelURL: ccTunnelURL,
		httpClient:  &http.Client{},
	}
}

// ResolveUsername calls cc-tunnel /app-auth/me with the request's Bearer token.
func (r *CCTunnelTokenResolver) ResolveUsername(req *http.Request) (string, error) {
	token, ok := bearerToken(req)
	if !ok {
		return "", ErrUnauthorized
	}

	meURL := r.ccTunnelURL + "/app-auth/me"
	meReq, err := http.NewRequestWithContext(req.Context(), http.MethodGet, meURL, nil)
	if err != nil {
		return "", fmt.Errorf("NewRequest: %w", err)
	}
	meReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := r.httpClient.Do(meReq)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", meURL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			_ = err // best-effort close
		}
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from cc-tunnel: %d", resp.StatusCode)
	}

	var body struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if body.User.Name == "" {
		return "", fmt.Errorf("empty username in response")
	}
	return body.User.Name, nil
}
