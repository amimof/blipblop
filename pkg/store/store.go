package store

// import (
// 	"fmt"
//
// 	"github.com/amimof/blipblop/pkg/labels"
// )
//
// type Store interface {
// 	Exists(key string) (bool, error)
// 	Get(key string) (string, error)
// 	Set(data string, key string) error
// 	Delete(key string) error
// 	Map() map[string]string
// }
//
// var labelPrefix = "blipblop"
//
// // labelStore is a store that uses container labels for storage
// type labelStore struct {
// 	store labels.Label
// }
//
// func (s *labelStore) Exists(key string) (bool, error) {
// 	if v := s.store.Get(fmt.Sprintf("%s/%s", labelPrefix, key)); v != nil {
// 		return true, nil
// 	}
// 	return false, nil
// }
//
// func (s *labelStore) Set(key, data string) error {
// 	s.store.Set(fmt.Sprintf("%s/%s", labelPrefix, key), data)
// 	return nil
// }
//
// func (s *labelStore) Get(key string) (string, error) {
// 	if v := s.store.Get(fmt.Sprintf("%s/%s", labelPrefix, key)); v != nil {
// 		return *v, nil
// 	}
// 	return "", nil
// }
//
// func (s *labelStore) Delete(key string) error {
// 	s.store.Delete(fmt.Sprintf("%s/%s", labelPrefix, key))
// 	return nil
// }
//
// func (s *labelStore) Map() map[string]string {
// 	return s.store
// }
//
// func NewLabelStoreFrom(l labels.Label) Store {
// 	if l == nil {
// 		l = labels.New()
// 	}
// 	return &labelStore{
// 		store: l,
// 	}
// }
//
// func NewLabelStore() Store {
// 	return &labelStore{
// 		store: labels.New(),
// 	}
// }
