package handlers

import (
	"mime/multipart"
	stdlibhttp "net/http"
	"strings"

	"social-network/backend/internal/http/response"
	"social-network/backend/internal/social"
)

func (h SocialHandler) HandleGroupPosts(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	posts, err := h.service.GroupPosts(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("groupID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"posts": posts,
	})
}

func (h SocialHandler) HandleCreateGroupPost(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var (
		payload struct {
			Body string `json:"body"`
		}
		images []*multipart.FileHeader
	)

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeError(w, stdlibhttp.StatusBadRequest, "Group post form data is invalid.", nil)
			return
		}

		payload.Body = r.FormValue("body")
		if r.MultipartForm != nil {
			images = r.MultipartForm.File["images"]
		}
	} else {
		if err := decodeJSON(r, &payload); err != nil {
			writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
			return
		}
	}

	post, err := h.service.CreateGroupPost(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("groupID")),
		social.CreateGroupPostInput{
			Body:   payload.Body,
			Images: images,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"post": post,
	})
}
