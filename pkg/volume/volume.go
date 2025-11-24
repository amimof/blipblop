// Package volume provides an API for working with volumes on nodes
package volume

type Manager interface {
	Create(volume Volume) error
	Get(name string) (Volume, error)
	Delete(name string) error
	List() ([]Volume, error)
}

type Volume interface {
	Name() string
	Location() string
}
