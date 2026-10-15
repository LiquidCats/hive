package values

type SecretVersion []byte

func (v SecretVersion) String() string {
	return string(v)
}

func (v SecretVersion) Bytes() []byte {
	return v
}
