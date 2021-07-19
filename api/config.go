package api

type Config struct {
	BindAddr   string `toml:"bind_addr"`
	DBSize     int    `toml:"db_size"`
	DBFileName string `toml:"file_name"`
}

func NewConfig() *Config {
	return &Config{
		BindAddr:   ":8080",
		DBSize:     0,
		DBFileName: "db.dat",
	}
}
