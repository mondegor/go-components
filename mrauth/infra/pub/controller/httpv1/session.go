package httpv1

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-webcore/mrserver"
	"github.com/mondegor/go-webcore/mrserver/request"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/model"
	"github.com/mondegor/go-components/mrauth/validate"
)

const (
	sessionsURL      = "/v1/sessions"
	sessionsCloseURL = "/v1/sessions/close"
)

type (
	// Session - контроллер управления открытыми сессиями пользователя.
	Session struct {
		parser  validate.RequestParser
		sender  mrserver.ResponseSender
		useCase sessionUseCase
	}

	sessionUseCase interface {
		GetList(ctx context.Context, userID uuid.UUID, currentAccessToken string) ([]dto.UserSession, error)
		Close(ctx context.Context, userID uuid.UUID, sessionIDs []uint32) error
	}
)

// NewSession - создаёт объект Session.
func NewSession(
	parser validate.RequestParser,
	sender mrserver.ResponseSender,
	useCase sessionUseCase,
) *Session {
	return &Session{
		parser:  parser,
		sender:  sender,
		useCase: useCase,
	}
}

// Handlers - возвращает обработчики контроллера Session.
func (ht *Session) Handlers() []mrserver.HttpHandler {
	return []mrserver.HttpHandler{
		{Method: http.MethodGet, URL: sessionsURL, Permission: mraccess.PermissionAnyUser, Func: ht.GetList},
		{Method: http.MethodPost, URL: sessionsCloseURL, Permission: mraccess.PermissionAnyUser, Func: ht.Close},
	}
}

// GetList - возвращает список открытых сессий текущего пользователя.
func (ht *Session) GetList(w http.ResponseWriter, r *http.Request) error {
	list, err := ht.useCase.GetList(r.Context(), ht.parser.UserID(r), request.AccessToken(r))
	if err != nil {
		return err
	}

	items := make([]model.UserSessionResponse, 0, len(list))
	for _, item := range list {
		items = append(
			items,
			model.UserSessionResponse{
				SessionID:  fmt.Sprintf("%08x", item.SessionID),
				AppName:    item.AppName,
				DeviceName: item.DeviceName,
				LastIP:     item.LastIP,
				Location:   item.Location,
				IsCurrent:  item.IsCurrent,
			},
		)
	}

	return ht.sender.Send(w, http.StatusOK, items)
}

// Close - закрывает указанные сессии текущего пользователя.
func (ht *Session) Close(w http.ResponseWriter, r *http.Request) error {
	req := model.CloseSessionsRequest{}

	if err := ht.parser.Validate(r, &req); err != nil {
		return err
	}

	sessionIDs := make([]uint32, 0, len(req.SessionIDs))
	for _, hash := range req.SessionIDs {
		id, err := strconv.ParseUint(hash, 16, 32)
		if err != nil {
			return errors.WithCustomCode(errors.ErrIncorrectInputData.New(err), "hashes")
		}

		sessionIDs = append(sessionIDs, uint32(id))
	}

	if err := ht.useCase.Close(r.Context(), ht.parser.UserID(r), sessionIDs); err != nil {
		return err
	}

	return ht.sender.SendNoContent(w)
}
