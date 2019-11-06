package skymarshal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/skymarshal/dexserver"
	"github.com/concourse/concourse/skymarshal/legacyserver"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/skymarshal/skyserver"
	"github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/flag"
)

type Config struct {
	Logger      lager.Logger
	Flags       skycmd.AuthFlags
	ExternalURL *url.URL
	HTTPClient  *http.Client
	Storage     storage.Storage
}

type Server struct {
	http.Handler
}

func NewServer(config *Config) (*Server, error) {

	signingKey, err := loadOrGenerateSigningKey(config.Flags.SigningKey)
	if err != nil {
		return nil, err
	}

	clientID := "skymarshal"
	clientSecretBytes := sha256.Sum256(signingKey.D.Bytes())
	clientSecret := fmt.Sprintf("%x", clientSecretBytes[:])

	issuerPath := "/sky/issuer"
	issuerURL := config.ExternalURL.String() + issuerPath
	redirectURL := config.ExternalURL.String() + "/sky/callback"

	tokenVerifier := token.NewVerifier(clientID, issuerURL)
	tokenMiddleware := token.NewMiddleware(config.Flags.SecureCookies)

	skyServer, err := skyserver.NewSkyServer(&skyserver.SkyConfig{
		Logger:          config.Logger.Session("sky"),
		TokenVerifier:   tokenVerifier,
		TokenMiddleware: tokenMiddleware,
		SigningKey:      signingKey,
		DexIssuerURL:    issuerURL,
		DexClientID:     clientID,
		DexClientSecret: clientSecret,
		DexRedirectURL:  redirectURL,
		DexHTTPClient:   config.HTTPClient,
		SecureCookies:   config.Flags.SecureCookies,
	})
	if err != nil {
		return nil, err
	}

	dexServer, err := dexserver.NewDexServer(&dexserver.DexConfig{
		Logger:     config.Logger.Session("dex"),
		Flags:      config.Flags,
		IssuerURL:  issuerURL,
		WebHostURL: issuerPath,
		SigningKey: signingKey,
		Storage:    config.Storage,
		Clients: []*dexserver.DexClient{
			{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
			},
			{
				ClientID:     "fly",
				ClientSecret: "Zmx5Cg==",
				RedirectURL:  redirectURL,
			},
			{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	legacyServer, err := legacyserver.NewLegacyServer(&legacyserver.LegacyConfig{
		Logger: config.Logger.Session("legacy"),
	})
	if err != nil {
		return nil, err
	}

	handler := http.NewServeMux()
	handler.Handle("/sky/issuer/", dexServer)
	handler.Handle("/sky/", skyserver.NewSkyHandler(skyServer))
	handler.Handle("/auth/", legacyServer)
	handler.Handle("/login", legacyServer)
	handler.Handle("/logout", legacyServer)

	return &Server{handler}, nil
}

func loadOrGenerateSigningKey(keyFlag *flag.PrivateKey) (*rsa.PrivateKey, error) {
	if keyFlag != nil && keyFlag.PrivateKey != nil {
		return keyFlag.PrivateKey, nil
	}

	return rsa.GenerateKey(rand.Reader, 2048)
}
