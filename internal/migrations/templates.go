package migrations

import (
	"fmt"
	"regexp"
	"strings"
)

const migrationMatch = `\{\{([^}]+)\}\}`

type Template struct {
	Name    string
	Content *string
}

func ParseTemplates(content *string, templates []*Template) {
	re := regexp.MustCompile(migrationMatch)

	matches := re.FindAllStringSubmatch(*content, -1)

	for _, match := range matches {

		matchContent := strings.TrimSpace(match[1])

		values := strings.Split(matchContent, ",")

		name := strings.TrimSpace(values[0])

		for _, template := range templates {
			if template.Name != name {
				continue
			}

			newTemplateContent := template.Content
			for i, value := range values[1:] {
				*newTemplateContent = strings.Replace(*newTemplateContent, fmt.Sprintf("$%d", i+1), strings.TrimSpace(value), -1)
			}

			*content = strings.Replace(*content, match[0], *newTemplateContent, -1)

			break
		}
	}
}
