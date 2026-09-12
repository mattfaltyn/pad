//go:build !unix

package cli

func lockRegistry(string) (unlock func(), err error) {
	return func() {}, nil
}
