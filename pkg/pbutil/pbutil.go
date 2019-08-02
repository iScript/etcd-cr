package pbutil

type Marshaler interface {
	Marshal() (data []byte, err error)
}

type Unmarshaler interface {
	Unmarshal(data []byte) error
}
