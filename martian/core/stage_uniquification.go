package core

import "os"

const uniquifiedDirectoriesEnv = "MRO_UNIQUIFIED_DIRECTORIES"

func shouldDisableUniquification() bool {
	return shouldDisableUniquificationValue(os.Getenv(uniquifiedDirectoriesEnv))
}
