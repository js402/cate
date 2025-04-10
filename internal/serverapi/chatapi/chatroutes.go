package chatapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/runtimestate"
	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/services/chatservice"
)

func AddChatRoutes(mux *http.ServeMux, _ *serverops.Config, chatManager *chatservice.Service, stateService *runtimestate.State) {
	h := &chatManagerHandler{manager: chatManager, stateService: stateService}

	mux.HandleFunc("POST /chats", h.createChat)
	mux.HandleFunc("POST /chats/{id}/chat/{model}", h.chat)
	mux.HandleFunc("POST /chats/{id}/instruction", h.addInstruction)
	mux.HandleFunc("GET /chats/{id}", h.history)
	mux.HandleFunc("GET /chats", h.listChats)
}

type chatManagerHandler struct {
	manager      *chatservice.Service
	stateService *runtimestate.State
}

type newChatInstanceRequest struct {
	Model   string `json:"model"`
	Subject string `json:"subject"`
}

func (h *chatManagerHandler) createChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req, err := serverops.Decode[newChatInstanceRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}
	chatID, err := h.manager.NewInstance(ctx, req.Subject, req.Model)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	resp := map[string]string{
		"id": chatID.String(),
	}
	_ = serverops.Encode(w, r, http.StatusCreated, resp)
}

type instructionRequest struct {
	Instruction string `json:"instruction"`
}

func (h *chatManagerHandler) addInstruction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	chatID, err := uuid.Parse(idStr)
	if err != nil {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue(err.Error()), serverops.CreateOperation)
		return
	}

	req, err := serverops.Decode[instructionRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	err = h.manager.AddInstruction(ctx, chatID, req.Instruction)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	resp := map[string]string{}
	_ = serverops.Encode(w, r, http.StatusOK, resp)
}

type chatRequest struct {
	Message string `json:"message"`
}

func (h *chatManagerHandler) chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	model := r.PathValue("model")
	chatID, err := uuid.Parse(idStr)
	if err != nil {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue(err.Error()), serverops.ServerOperation)
		return
	}

	req, err := serverops.Decode[chatRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ServerOperation)
		return
	}

	reply, err := h.manager.Chat(ctx, chatID, req.Message, model)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ServerOperation)
		return
	}

	resp := map[string]string{
		"response": reply,
	}
	_ = serverops.Encode(w, r, http.StatusOK, resp)
}

func (h *chatManagerHandler) history(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	chatID, err := uuid.Parse(idStr)
	if err != nil {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue("missing required id parameter"), serverops.ListOperation)
		return
	}

	history, err := h.manager.GetChatHistory(ctx, chatID)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.GetOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, history)
}

func (h *chatManagerHandler) listChats(w http.ResponseWriter, r *http.Request) {
	chats, err := h.manager.ListChats(r.Context())
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ListOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, chats)
}
