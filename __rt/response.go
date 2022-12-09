package __rt

type Response interface {
	Err() error
	ScanValues(...any) error
	FromBytes(string, []byte) error
	Break() bool
}
