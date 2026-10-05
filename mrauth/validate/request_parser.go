package validate

import (
	"github.com/mondegor/go-webcore/mrserver/request"
	"github.com/mondegor/go-webcore/mrserver/request/parser"
)

// TODO: перенести в web-core

type (
	// RequestParser - агрегирующий интерфейс парсеров HTTP-запроса.
	RequestParser interface {
		request.ParserInt64
		request.ParserUint64
		request.ParserString
		request.ParserUUID
		request.ParserValidate
		request.ParserClient
		request.ParserUser
		request.ParserLocale
		request.ParserTimeZone
		request.ParserListCursor
	}

	// Parser - реализация RequestParser на основе парсеров go-webcore.
	Parser struct {
		*parser.Int64
		*parser.Uint64
		*parser.String
		*parser.UUID
		*parser.Validator
		*parser.Client
		*parser.User
		*parser.Locale
		*parser.TimeZone
		*parser.ListCursor
	}
)

// NewParser - создаёт объект Parser.
func NewParser(
	p1 *parser.Int64,
	p2 *parser.Uint64,
	p3 *parser.String,
	p4 *parser.UUID,
	p5 *parser.Validator,
	p6 *parser.Client,
	p7 *parser.User,
	p8 *parser.Locale,
	p9 *parser.TimeZone,
	p10 *parser.ListCursor,
) *Parser {
	return &Parser{
		Int64:      p1,
		Uint64:     p2,
		String:     p3,
		UUID:       p4,
		Validator:  p5,
		Client:     p6,
		User:       p7,
		Locale:     p8,
		TimeZone:   p9,
		ListCursor: p10,
	}
}
