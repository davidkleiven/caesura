package web

import (
	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
	"gopkg.in/yaml.v2"
)

type Translator struct {
	mapping map[string]map[string]string
}

func (t *Translator) MustGet(lang, field string) string {
	fallback := "en"
	languageMap, ok := t.mapping[lang]
	if !ok {
		languageMap, ok = t.mapping[fallback]
		if !ok {
			panic("Fallback language must exist")
		}
	}
	translation, ok := languageMap[field]
	if !ok {
		return field
	}
	return translation
}

func NewTranslator() *Translator {
	data := utils.Must(templatesFS.ReadFile("templates/translations.yml"))
	var mapping map[string]map[string]string
	pkg.PanicOnErr(yaml.Unmarshal(data, &mapping))
	return &Translator{mapping: mapping}
}

// Global translator
var translator = NewTranslator()
