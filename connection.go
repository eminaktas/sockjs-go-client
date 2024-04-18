package sockjsclient

type Connection interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	ForceClose()
}
