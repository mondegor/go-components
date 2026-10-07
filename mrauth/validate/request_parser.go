package validate

import (
	"github.com/mondegor/go-webcore/mrserver/request"
	"github.com/mondegor/go-webcore/mrserver/request/parser"
	"github.com/mondegor/go-webcore/mrserver/request/validate"
)

type (
	// RequestParser - агрегирующий интерфейс парсеров HTTP-запроса модуля:
	// базовые парсеры, курсор списков и парсеры контекста запроса.
	RequestParser interface {
		validate.RequestParser
		validate.RequestContextParser
		request.ParserListCursor
	}

	// Parser - реализация RequestParser, соединяющая базовый парсер,
	// парсер курсора и парсер контекста запроса go-webcore.
	Parser struct {
		*validate.Parser
		*validate.ContextParser
		*parser.ListCursor
	}
)

// NewParser - создаёт объект Parser.
func NewParser(
	baseParser *validate.Parser,
	contextParser *validate.ContextParser,
	listCursorParser *parser.ListCursor,
) *Parser {
	return &Parser{
		Parser:        baseParser,
		ContextParser: contextParser,
		ListCursor:    listCursorParser,
	}
}

// WithListCursor - возвращает копию парсера с заменённым парсером курсорной пагинации.
func (p *Parser) WithListCursor(listCursor *parser.ListCursor) *Parser {
	c := *p
	c.ListCursor = listCursor

	return &c
}
