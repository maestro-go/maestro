package filesystem

import (
	"crypto/md5"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/maestro-go/maestro/core/enums"
	internalConf "github.com/maestro-go/maestro/internal/conf"
	"github.com/maestro-go/maestro/internal/migrations"
)

// LoadObjectsFromFiles reads migration and hook files from the specified directories.
//
// This function processes files in the given directories to load migration and hook objects.
// It uses the provided configuration to determine which migrations and hooks should be included,
// avoiding unnecessary memory usage. If a file contains templates, they are replaced with actual
// content. For up migration files, an MD5 checksum is generated for the final content (after the templates process).
//
// Notes:
//   - Files are processed concurrently for better performance.
//   - Mutexes ensure thread-safe updates to the migration and hook maps.
//   - Only migrations and hooks matching the configuration criteria are loaded.
func LoadObjectsFromFiles(config *conf.MigrationConfig) (
	map[enums.MigrationType][]*migrations.Migration, map[enums.HookType][]*migrations.Hook, []error) {

	templates, errs := loadTemplates(config.Locations)
	if len(errs) > 0 {
		return nil, nil, errs
	}

	migrationsO := make(map[enums.MigrationType][]*migrations.Migration)
	hooksO := make(map[enums.HookType][]*migrations.Hook)

	muM := new(sync.Mutex) // Locks the access to migrations slice
	muH := new(sync.Mutex) // Locks the access to hooks slice

	for _, migrationDir := range config.Locations {
		entries, err := os.ReadDir(migrationDir)
		if err != nil {
			return nil, nil, []error{err}
		}

		loadObjectsErrs := make([]error, 0)
		wg := new(sync.WaitGroup)
		for _, entry := range entries {
			wg.Add(1)
			go func(entry fs.DirEntry) {
				defer wg.Done()

				migration, isMigration, err := checkAndLoadMigrationInfo(entry.Name())
				if err != nil {
					loadObjectsErrs = append(loadObjectsErrs, err)
					return
				}

				if isMigration {
					if isToAddMigration(migration, config) {
						content, err := loadFileContent(filepath.Join(migrationDir, entry.Name()), templates)
						if err != nil {
							loadObjectsErrs = append(loadObjectsErrs, err)
							return
						}

						migration.Content = content

						if migration.Type == enums.MIGRATION_UP {
							md5Checksum := generateMd5Checksum(content)
							migration.Checksum = &md5Checksum
						}

						muM.Lock()
						migrationsO[migration.Type] = append(migrationsO[migration.Type], migration)
						muM.Unlock()
					}
					return
				}

				hook, isHook, err := checkAndLoadHookInfo(entry.Name())
				if err != nil {
					loadObjectsErrs = append(loadObjectsErrs, err)
					return
				}

				if isHook && isToAddHook(hook, config) {
					content, err := loadFileContent(filepath.Join(migrationDir, entry.Name()), templates)
					if err != nil {
						loadObjectsErrs = append(loadObjectsErrs, err)
						return
					}

					hook.Content = content

					muH.Lock()
					hooksO[hook.Type] = append(hooksO[hook.Type], hook)
					muH.Unlock()
				}
			}(entry)
		}

		wg.Wait()
		if len(loadObjectsErrs) > 0 {
			return nil, nil, loadObjectsErrs
		}
	}

	sortMigrations(&migrationsO)
	sortHooks(&hooksO)

	return migrationsO, hooksO, nil
}

// loadTemplates loads migration templates from the specified directories.
//
// This function iterates over the provided list of directory paths, reads all files
// within each directory, and identifies files that match the template naming
// pattern. For each matching file, it extracts the template name and content,
// creating a template object.
// These objects are collected into a slice, which is returned along with any errors
// encountered during the process.
func loadTemplates(migrationsDirs []string) ([]*migrations.Template, []error) {
	templatesO := make([]*migrations.Template, 0)

	re := regexp.MustCompile(internalConf.TEMPLATE_REGEX)

	mu := new(sync.Mutex) // Blocks access to slice

	for _, migrationDir := range migrationsDirs {
		entries, err := os.ReadDir(migrationDir)
		if err != nil {
			return nil, []error{err}
		}

		loadFilesErrs := make([]error, 0)
		wg := new(sync.WaitGroup)
		for _, entry := range entries {
			wg.Add(1)
			go func(entry fs.DirEntry) {
				defer wg.Done()

				matches := re.FindStringSubmatch(entry.Name())

				if matches == nil {
					return
				}

				templateName := matches[1]

				content, err := os.ReadFile(filepath.Join(migrationDir, entry.Name()))
				if err != nil {
					loadFilesErrs = append(loadFilesErrs, err)
				}

				contentStr := string(content)

				template := &migrations.Template{
					Name:    templateName,
					Content: &contentStr,
				}

				mu.Lock()
				templatesO = append(templatesO, template)
				mu.Unlock()
			}(entry)
		}

		wg.Wait()
		if len(loadFilesErrs) > 0 {
			return templatesO, loadFilesErrs
		}
	}

	return templatesO, nil
}

// checkAndLoadMigrationInfo determines if the given file name corresponds to a migration and extracts its details.
//
// This function iterates over a predefined map of migration types to their corresponding regex patterns,
// attempting to match the provided file name against each pattern. If a match is found, it extracts the
// migration's version and description from the file name and returns a Migration object with these details.
//
// Notes:
//   - The function uses a map (`enums.MapMigrationTypeToRegex`) that associates migration types with regex
//     patterns to identify the type of migration.
//   - If the file name does not match any regex pattern, the function returns nil, false, and no error.
func checkAndLoadMigrationInfo(fileName string) (*migrations.Migration, bool, error) {
	for migrationType, regex := range enums.MapMigrationTypeToRegex {
		re := regexp.MustCompile(regex)

		matches := re.FindStringSubmatch(fileName)

		if matches != nil {
			version := uint16(0)
			description := string("")

			versionStr := matches[1]
			v, err := strconv.ParseUint(versionStr, 10, 16)
			if err != nil {
				return nil, false, err
			}

			version = uint16(v)
			description = matches[2]

			migration := &migrations.Migration{
				Type:        migrationType,
				Version:     version,
				Description: description,
			}

			return migration, true, nil
		}
	}

	return nil, false, nil
}

func isToAddMigration(migration *migrations.Migration, config *conf.MigrationConfig) bool {
	return migration.Type == enums.MIGRATION_UP ||
		migration.Type == enums.MIGRATION_DOWN && config.Down
}

// checkAndLoadHookInfo determines if the given file name corresponds to a hook and extracts its details.
//
// This function iterates over a predefined map of hook types to their corresponding regex patterns,
// attempting to match the provided file name against each pattern. If a match is found, it extracts the
// hook's order, description and version (for BEV and AEV hooks) from the file name and returns a Hook object
// with these details.
//
// Notes:
//   - The function uses a map (`enums.MapHookTypeToRegex `) that associates hook types with regex
//     patterns to identify the type of hook.
//   - If the file name does not match any regex pattern, the function returns nil, false, and no error.
func checkAndLoadHookInfo(fileName string) (*migrations.Hook, bool, error) {
	for hookType, regex := range enums.MapHookTypeToRegex {
		re := regexp.MustCompile(regex)

		matches := re.FindStringSubmatch(fileName)

		if matches != nil {
			order := uint8(0)

			orderStr := matches[1]
			o, err := strconv.ParseUint(orderStr, 10, 8)
			if err != nil {
				return nil, false, err
			}

			order = uint8(o)

			hook := &migrations.Hook{
				Type:  hookType,
				Order: order,
			}

			if hookType == enums.HOOK_BEFORE_VERSION || hookType == enums.HOOK_AFTER_VERSION {
				versionStr := matches[2]
				v, err := strconv.ParseUint(versionStr, 10, 16)
				if err != nil {
					return nil, false, err
				}

				hook.Version = uint16(v)
			}

			return hook, true, nil
		}
	}

	return nil, false, nil
}

func isToAddHook(hook *migrations.Hook, config *conf.MigrationConfig) bool {
	if config.Down {
		return hook.Type == enums.HOOK_REPEATABLE_DOWN && config.UseRepeatable
	}

	isToAdd := false
	switch hook.Type {
	case enums.HOOK_BEFORE:
		isToAdd = config.UseBefore
	case enums.HOOK_BEFORE_EACH:
		isToAdd = config.UseBeforeEach
	case enums.HOOK_BEFORE_VERSION:
		isToAdd = config.UseBeforeVersion
	case enums.HOOK_AFTER:
		isToAdd = config.UseAfter
	case enums.HOOK_AFTER_EACH:
		isToAdd = config.UseAfterEach
	case enums.HOOK_AFTER_VERSION:
		isToAdd = config.UseAfterVersion
	case enums.HOOK_REPEATABLE:
		isToAdd = config.UseRepeatable
	}
	return isToAdd
}

func loadFileContent(filePath string, templates []*migrations.Template) (*string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	contentStr := string(content)

	migrations.ParseTemplates(&contentStr, templates)

	return &contentStr, nil
}

func generateMd5Checksum(content *string) string {
	md5CheckSum := md5.Sum([]byte(*content))

	return hex.EncodeToString(md5CheckSum[:])
}

func sortMigrations(groupedMigrations *map[enums.MigrationType][]*migrations.Migration) {
	for migrationsType := range *groupedMigrations {
		sort.Slice((*groupedMigrations)[migrationsType], func(i, j int) bool {
			if migrationsType == enums.MIGRATION_DOWN {
				return (*groupedMigrations)[migrationsType][i].Version > (*groupedMigrations)[migrationsType][j].Version
			}
			return (*groupedMigrations)[migrationsType][i].Version < (*groupedMigrations)[migrationsType][j].Version
		})
	}
}

func sortHooks(groupedHooks *map[enums.HookType][]*migrations.Hook) {
	for hookType := range *groupedHooks {
		sort.Slice((*groupedHooks)[hookType], func(i, j int) bool {
			return (*groupedHooks)[hookType][i].Order < (*groupedHooks)[hookType][j].Order
		})
	}
}
