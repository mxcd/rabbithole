package authentication

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	basicauth "github.com/mxcd/go-basicauth"
	"github.com/rs/zerolog/log"
)

type Options struct {
	Engine               *gin.Engine
	ApiBaseUrl           string
	SessionSigningKey    []byte
	SessionEncryptionKey []byte
	DefaultAdminPassword string
	IsDev                bool
}

func Init(options *Options) (*basicauth.Handler, error) {
	storage := basicauth.NewMemoryStorage()

	if err := ensureAdminUser(storage, options.DefaultAdminPassword); err != nil {
		return nil, err
	}

	settings := basicauth.DefaultSettings()
	settings.SessionSecretKey = options.SessionSigningKey
	settings.SessionEncryptionKey = options.SessionEncryptionKey
	settings.CookieSecure = !options.IsDev
	settings.EnableEmailLogin = false
	settings.EnableUsernameLogin = true
	settings.PathRules = []basicauth.PathRule{
		{Type: basicauth.PublicPathPrefix, Path: "/", Access: basicauth.PathAccessPublic},
		{Type: basicauth.PublicPathPrefix, Path: options.ApiBaseUrl, Access: basicauth.PathAccessPrivate},
		{Type: basicauth.PublicPathExact, Path: options.ApiBaseUrl + "/health", Access: basicauth.PathAccessPublic},
		{Type: basicauth.PublicPathExact, Path: options.ApiBaseUrl + "/tunnel/connect", Access: basicauth.PathAccessPublic},
	}

	handler, err := basicauth.NewHandler(&basicauth.Options{
		Engine:                options.Engine,
		AuthenticationBaseUrl: options.ApiBaseUrl + "/auth",
		Storage:               storage,
		Settings:              settings,
	})
	if err != nil {
		return nil, err
	}

	if err := handler.RegisterRoutes(); err != nil {
		return nil, err
	}

	return handler, nil
}

func ensureAdminUser(storage *basicauth.MemoryStorage, password string) error {
	_, err := storage.GetUserByUsername("admin")
	if err == nil {
		return nil
	}

	hash, err := basicauth.HashPassword(password, basicauth.DefaultPasswordHashingParams)
	if err != nil {
		return err
	}

	username := "admin"
	now := time.Now()
	user := &basicauth.User{
		ID:           uuid.New(),
		Username:     &username,
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := storage.CreateUser(user); err != nil {
		return err
	}

	log.Info().Msg("default admin user created")
	return nil
}
