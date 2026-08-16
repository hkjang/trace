package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/hkjang/trace/internal/domain"
	"golang.org/x/oauth2"
)

func oidcClient(ctx context.Context, settings domain.OIDCSettings) (*oidc.Provider, *oauth2.Config, error) {
	provider, err := oidc.NewProvider(ctx, settings.IssuerURL)
	if err != nil {
		return nil, nil, err
	}
	config := &oauth2.Config{ClientID: settings.ClientID, ClientSecret: settings.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: settings.BaseURL + "/api/v1/auth/oidc/callback", Scopes: strings.Fields(settings.Scopes)}
	return provider, config, nil
}
func storeRandomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
