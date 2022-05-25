package labels

// type Label interface {
// 	Value(string) *string
// 	Set(string, string)
// }

type Label map[string]string

var DefaultContainerLabels = []string{"blipblop.io/uuid"}

func (l *Label) Value(key string) *string {
	if val, ok := (*l)[key]; ok {
		return &val
	}
	return nil
}

func (l *Label) Set(key string, value string) {
	(*l)[key] = value
}

func New() Label {
	return Label{}
}