package schema

type AttributeError struct {
	attributeAddr string // dotted syntax
	origErr       error
}

func (ae *AttributeError) Error() string {
	return ae.origErr.Error()
}

func (ae *AttributeError) Addr() string {
	return ae.attributeAddr
}

func NewAttributeError(addr string, origErr error) *AttributeError {
	return &AttributeError{addr, origErr}
}
