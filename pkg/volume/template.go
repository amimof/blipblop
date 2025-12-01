package volume

import (
	"context"
	"os"
	"path"

	v1 "github.com/amimof/blipblop/pkg/client/volume/v1"
)

var _ Driver = &templateDriver{}

type templateDriver struct {
	client   v1.ClientV1
	rootPath string
}

// Create implements Driver.
func (t *templateDriver) Create(ctx context.Context, name string) (Volume, error) {
	vol, err := t.client.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	p := path.Join(t.rootPath, vol.GetMeta().GetName())

	// Make sure root path exists
	err = os.MkdirAll(p, 0o750)
	if err != nil {
		return nil, err
	}

	data := vol.GetConfig().GetTemplate().GetData()
	fpath := path.Join(p, vol.GetConfig().GetTemplate().GetName())

	// Create file
	f, err := os.Create(fpath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	// Write to the file
	err = os.WriteFile(fpath, []byte(data), 0o666)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return &FSVolume{
		id:       ID(name),
		size:     stat.Size(),
		location: fpath,
	}, nil
}

// Delete implements Driver.
func (t *templateDriver) Delete(ctx context.Context, name string) error {
	vol, err := t.client.Get(ctx, name)
	if err != nil {
		return err
	}

	p := path.Join(t.rootPath, vol.GetMeta().GetName())

	return os.RemoveAll(p)
}

// Get implements Driver.
func (t *templateDriver) Get(ctx context.Context, name string) (Volume, error) {
	vol, err := t.client.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	p := path.Join(t.rootPath, vol.GetMeta().GetName())
	fpath := path.Join(p, vol.GetConfig().GetTemplate().GetName())

	stat, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	return &FSVolume{
		id:       ID(vol.GetMeta().GetName()),
		size:     stat.Size(),
		location: fpath,
	}, err

	// return nil, fmt.Errorf("volume not found")
}

// List implements Driver.
func (t *templateDriver) List(ctx context.Context) ([]Volume, error) {
	result := []Volume{}
	vols, err := t.client.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, vol := range vols {
		driverType := GetDriverType(vol)
		if driverType == DriverTypeTemplate {
			result = append(result, &FSVolume{
				id:       ID(vol.GetMeta().GetName()),
				size:     0,
				location: path.Join(t.rootPath, vol.GetMeta().GetName()),
			})
		}
	}

	return result, nil
}

func NewTemplateDriver(client v1.ClientV1, rootPath string) Driver {
	return &templateDriver{
		client:   client,
		rootPath: rootPath,
	}
}
