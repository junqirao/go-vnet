package config

type (
	MappedConfig interface {
		Get(key string) (val any, ok bool)
		Set(key string, val any)
	}
	mappedConfig struct {
		values map[string]any
	}
)

func NewMappedConfig() MappedConfig {
	return &mappedConfig{
		values: make(map[string]any),
	}
}

func (m *mappedConfig) Get(key string) (val any, ok bool) {
	if m.values == nil {
		return
	}
	val, ok = m.values[key]
	return
}

func (m *mappedConfig) Set(key string, val any) {
	if m.values == nil {
		m.values = make(map[string]any)
	}
	m.values[key] = val
}

func GetMappedConfig[T any](m MappedConfig, key string, def ...T) (val T) {
	val = *new(T)
	if m == nil {
		return
	}
	v, ok := m.Get(key)
	if ok {
		val, ok = v.(T)
		if ok {
			return
		}
	}
	if len(def) > 0 {
		val = def[0]
	}
	return
}
