package optional

type Optional[Value any] struct {
	value *Value
	Valid bool
}

func WithValue[Value any](val *Value) Optional[Value] {
	if val != nil {
		return Optional[Value]{
			value: val,
			Valid: true,
		}
	} else {
		return Optional[Value]{}
	}
}
