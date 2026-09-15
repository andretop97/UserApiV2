package core

type PepperProvider interface {
	Current() (version int, secret []byte)
	Get(version int) (secret []byte, ok bool)
}
