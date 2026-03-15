package config

import "github.com/mxcd/go-config/config"

func InitConfig() error {
	err := config.LoadConfig([]config.Value{
		// logging config
		config.String("LOG_LEVEL").NotEmpty().Default("info"),

		// server config
		config.Bool("DEV").Default(false),
		config.Int("PORT").Default(8080),
		config.String("API_BASE_URL").Default("/api/v1"),

		// tunnel config
		config.String("BASE_DOMAIN").NotEmpty().Default("localhost"),
		config.Int("TUNNEL_TIMEOUT").Default(30),

		// tunnel auth config
		config.String("API_KEYS").Default("").Sensitive(),

		// dashboard auth config
		config.String("SESSION_SECRET_KEY").NotEmpty().Sensitive(),
		config.String("SESSION_ENCRYPTION_KEY").NotEmpty().Sensitive(),
		config.String("DEFAULT_ADMIN_PASSWORD").NotEmpty().Sensitive(),

		// ui config
		config.Bool("STATIC_HOSTING").Default(true),
		config.String("UI_PROXY_URL").NotEmpty().Default("http://localhost:9000"),

		// cors config
		config.StringArray("CORS_ALLOWED_ORIGINS").Default([]string{"http://localhost:8080"}),
	})
	return err
}
