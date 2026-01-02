package client

import (
	"errors"
	"fmt"
)

var (
	ErrCurrentNotSet   = errors.New("current server is not set")
	ErrCurrentNotFound = errors.New("current server not found")
	ErrServerNotFound  = errors.New("server not found")
)

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

func (c *Config) AddServer(srv *Server) error {
	err := srv.Validate()
	if err != nil {
		return err
	}

	// Ensure server.Name is unique
	if getServer(c.Servers, srv.Name) != nil {
		return fmt.Errorf("server %s already exists", srv.Name)
	}

	c.Servers = append(c.Servers, srv)
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
		return ErrServerNotFound
	}

	for _, s := range c.Servers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("server validation failed: %v", err)
		}
	}
	return nil
}

func (c *Config) CurrentServer() (*Server, error) {
	if c == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if c.Current == "" {
		return nil, ErrCurrentNotSet
	}
	srv := getServer(c.Servers, c.Current)
	if srv == nil {
		return nil, ErrCurrentNotFound
	}
	return srv, nil
}

func (s *Server) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if s.Address == "" {
		return fmt.Errorf("address cannot be empty")
	}

	if err := s.TLSConfig.Validate(); err != nil {
		return err
	}

	return nil
}

func (t *TLSConfig) Validate() error {
	if t == nil {
		return nil
	}
	if !t.Insecure {
		if t.CA == "" {
			return fmt.Errorf("tls.ca cannot be empty when insecure=false")
		}
		if (t.Certificate == "") != (t.Key == "") {
			return fmt.Errorf("tls.certificate and tls.key must both be set or both empty")
		}
	}
	return nil
}
