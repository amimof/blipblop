package labels

// type Label interface {
// 	Value(string) *string
// 	Set(string, string)
// }

type Label map[string]string

var DefaultContainerLabels = []string{"blipblop.io/uuid"}

func (l *Label) Get(key string) string {
	if val, ok := (*l)[key]; ok {
		return val
	}
	return ""
}

func (l *Label) Set(key string, value string) {
	(*l)[key] = value
}

func (l *Label) Delete(key string) {
	delete(*l, key)
}

func (l *Label) AppendMap(m map[string]string) {
	for k, v := range m {
		l.Set(k, v)
	}
}

func New() Label {
	return Label{}
}
