package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTemplatesWithoutValues(t *testing.T) {
	content := "EXAMPLE {{test1}} {{test2}} {{test1}}"
	template1Content := "test_template_1"
	template2Content := "test_template_2"
	templates := []*Template{
		{
			Name:    "test1",
			Content: &template1Content,
		},
		{
			Name:    "test2",
			Content: &template2Content,
		},
	}

	expectedResult := "EXAMPLE test_template_1 test_template_2 test_template_1"

	ParseTemplates(&content, templates)

	assert.Equal(t, expectedResult, content)
}

func TestParseTemplatesWithValues(t *testing.T) {
	content := "EXAMPLE {{test1, 1, true, \"abc\"}}"
	template1Content := "test_template_1 $1, $2, $3"
	templates := []*Template{
		{
			Name:    "test1",
			Content: &template1Content,
		},
	}

	expectedResult := "EXAMPLE test_template_1 1, true, \"abc\""

	ParseTemplates(&content, templates)

	assert.Equal(t, expectedResult, content)
}
