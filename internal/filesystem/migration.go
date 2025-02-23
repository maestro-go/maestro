package filesystem

import (
	"os"
	"regexp"
	"strconv"

	"github.com/maestro-go/maestro/internal/conf"
)

func GetLatestVersionFromFiles(migrationsDirs []string) (uint16, error) {
	upRegex := regexp.MustCompile(conf.MIGRATION_REGEX)

	latest := uint16(0)
	for _, migrationDir := range migrationsDirs {
		entries, err := os.ReadDir(migrationDir)
		if err != nil {
			return 0, err
		}

		for _, entry := range entries {
			matches := upRegex.FindStringSubmatch(entry.Name())

			if matches != nil {
				version := uint16(0)

				v, err := strconv.ParseUint(matches[1], 10, 16)
				if err != nil {
					return 0, err
				}

				version = uint16(v)

				if version > latest {
					latest = version
				}
			}
		}
	}

	return latest, nil
}
