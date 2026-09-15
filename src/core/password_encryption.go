package core

type PasswordEncryption interface {
	Hash(password string) (encodedHash string, pepperVersion int, err error)
	Verify(password, encodedHash string, pepperVersion int) (bool, error)
}
