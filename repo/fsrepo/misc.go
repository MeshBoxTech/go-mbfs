package fsrepo

import (
	"os"

	config "mbfs/go-mbfs/gx/QmbK4EmM2Xx5fmbqK38TGP3PpY66r3tkXLZTcc7dF9mFwM/go-ipfs-config"
	homedir "mbfs/go-mbfs/gx/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"
)

// BestKnownPath returns the best known fsrepo path. If the ENV override is
// present, this function returns that value. Otherwise, it returns the default
// repo path.
func BestKnownPath() (string, error) {
	mbfsPath := config.DefaultPathRoot
	if os.Getenv(config.EnvDir) != "" {
		mbfsPath = os.Getenv(config.EnvDir)
	}
	mbfsPath, err := homedir.Expand(mbfsPath)
	if err != nil {
		return "", err
	}
	return mbfsPath, nil
}
