package pbutil

type Marshaler interface {
	Marshal() (data []byte, err error)
}

type Unmarshaler interface {
	Unmarshal(data []byte) error
}

// 获取bool的值，参数为bool指针， 返回值为指针的值和是否设置
func GetBool(v *bool) (vv bool, set bool) {
	if v == nil {
		return false, false
	}
	return *v, true // 有则获取指针的值
}

func Boolp(b bool) *bool { return &b }
