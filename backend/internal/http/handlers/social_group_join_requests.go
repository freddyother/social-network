package handlers

import (
	stdlibhttp "net/http"
	"strings"

	"social-network/backend/internal/http/response"
)

func (h SocialHandler) HandleGroupJoinRequests(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	requests, err := h.service.GroupJoinRequests(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("groupID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"requests": requests,
	})
}

func (h SocialHandler) HandleAcceptGroupJoinRequest(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	h.handleRespondGroupJoinRequest(w, r, true)
}

func (h SocialHandler) HandleDeclineGroupJoinRequest(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	h.handleRespondGroupJoinRequest(w, r, false)
}

func (h SocialHandler) handleRespondGroupJoinRequest(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request, accept bool) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	request, err := h.service.RespondToGroupJoinRequest(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("groupID")),
		strings.TrimSpace(r.PathValue("requestID")),
		accept,
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"request": request,
	})
}
