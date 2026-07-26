package model_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	// requestField - поле модели запроса, каким оно объявлено в исходниках пакета.
	requestField struct {
		name        string // "Модель.Поле", используется как имя подтеста
		isPointer   bool
		validateTag string
	}
)

// TestRequestModelTags - необязательное поле модели запроса объявляется указателем с тегом
// omitnil, а не строкой с omitempty: только так "поля нет" отличимо от "прислали пустым".
// С обычной строкой encoding/json даёт "" в обоих случаях, omitempty снимает такое поле
// с проверки, и пустая строка проходит валидацию вопреки minLength контракта.
// Парность min= и max= держит те же границы, что и minLength/maxLength в контракте.
//
// Правила проверяются по исходникам пакета, поэтому распространяются и на модели,
// добавленные позже.
func TestRequestModelTags(t *testing.T) {
	t.Parallel()

	fields := requestModelFields(t)
	require.NotEmpty(t, fields, "модели запросов не найдены, разбор исходников пакета сломан")

	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			t.Parallel()

			rules := strings.Split(field.validateTag, ",")

			if field.isPointer {
				assert.Equal(
					t,
					"omitnil",
					rules[0],
					"необязательное поле-указатель снимается с проверки только при nil, "+
						"поэтому цепочка тегов начинается с omitnil",
				)
			} else {
				assert.NotContains(
					t,
					rules,
					"omitempty",
					"необязательность выражается указателем с omitnil: omitempty на обычной "+
						"строке пропускает и пустую, присланную клиентом",
				)
			}

			if hasRule(rules, "max=") {
				assert.True(
					t,
					hasRule(rules, "min="),
					"у поля с max= должен быть парный min=, как maxLength и minLength в контракте",
				)
			}
		})
	}
}

// requestModelFields - собирает поля моделей запросов пакета из его исходников.
// Поля без тега validate пропускаются: проверять в них нечего.
func requestModelFields(t *testing.T) []requestField {
	t.Helper()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	fset := token.NewFileSet()

	var fields []requestField

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, parseErr)

		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(spec.Name.Name, "Request") {
				return true
			}

			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			fields = append(fields, structFields(t, spec.Name.Name, structure)...)

			return true
		})
	}

	return fields
}

// structFields - разбирает поля структуры модели, отбрасывая встроенные и без тега validate.
func structFields(t *testing.T, modelName string, structure *ast.StructType) []requestField {
	t.Helper()

	fields := make([]requestField, 0, len(structure.Fields.List))

	for _, field := range structure.Fields.List {
		if len(field.Names) == 0 || field.Tag == nil {
			continue
		}

		rawTag, err := strconv.Unquote(field.Tag.Value)
		require.NoError(t, err)

		validateTag := reflect.StructTag(rawTag).Get("validate")
		if validateTag == "" {
			continue
		}

		_, isPointer := field.Type.(*ast.StarExpr)

		for _, name := range field.Names {
			fields = append(fields, requestField{
				name:        modelName + "." + name.Name,
				isPointer:   isPointer,
				validateTag: validateTag,
			})
		}
	}

	return fields
}

// hasRule - сообщает, есть ли в цепочке тегов правило с указанным префиксом (напр. "min=").
func hasRule(rules []string, prefix string) bool {
	return slices.ContainsFunc(rules, func(rule string) bool {
		return strings.HasPrefix(rule, prefix)
	})
}
