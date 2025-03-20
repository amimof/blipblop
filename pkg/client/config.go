package client

import "fmt"

type Config struct {
	Version string    `mapstructure:"version"`
	Servers []*Server `mapstructure:"servers"`
	Current string    `mapstructure:"current"`
}

type Server struct {
	Name      string     `mapstructure:"name"`
	Address   string     `mapstructure:"address"`
	TLSConfig *TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	CA          string `mapstructure:"ca"`
	Certificate string `mapstructure:"certificate"`
	Key         string `mapstructure:"key"`
	Insecure    bool   `mapstructure:"insecure"`
}

func getServer(servers []*Server, name string) *Server {
	for _, server := range servers {
		if server.Name == name {
			return server
		}
	}
	return nil
}

func (c *Config) Validate() error {
	if c.Current == "" {
		return fmt.Errorf("current cannot be empty")
	}

	if len(c.Servers) <= 0 {
		return fmt.Errorf("no servers are configured")
	}

	if s := getServer(c.Servers, c.Current); s == nil {
		return fmt.Errorf("couldn't find a server matching %s", c.Current)
	}

	for _, s := range c.Servers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("server validation failed: %v", err)
		}
	}
	return nil
}

func (s *Server) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if s.Address == "" {
		return fmt.Errorf("address cannot be empty")
	}

	return nil
}

func (c *Config) CurrentServer() *Server {
	return getServer(c.Servers, c.Current)
}
