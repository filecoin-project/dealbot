package controller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
)

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}

func (c *Controller) getTasksHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")

	w.Header().Set("Content-Type", "application/json")

	enableCors(&w)

	tasks, err := c.db.GetAll(r.Context())
	if err != nil {
		log.Errorw("getTasks failed: backend", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(tasks)
}

func (c *Controller) popTaskHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: use a single SQL transaction to remove the need for a mutex here
	c.popTaskLk.Lock()
	defer c.popTaskLk.Unlock()

	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "pop task")
	defer logger.Debugw("request handled", "command", "pop task")

	w.Header().Set("Content-Type", "application/json")
	var req client.PopTaskRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Errorw("UpdateTaskRequest json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	task, err := c.db.AssignTask(r.Context(), req)
	if err != nil {
		log.Errorw("popTask failed: backend", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// If none are available, we return a JSON "null".
	json.NewEncoder(w).Encode(task)
}

func (c *Controller) updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "update task")
	defer logger.Debugw("request handled", "command", "update task")

	w.Header().Set("Content-Type", "application/json")
	var req client.UpdateTaskRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Errorw("UpdateTaskRequest json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	uuid := vars["uuid"]
	task, err := c.db.Update(r.Context(), uuid, req)
	if err != nil {
		log.Errorw("UpdateTaskRequest db update", "err", err.Error())
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func (c *Controller) newStorageTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "create task")
	defer logger.Debugw("request handled", "command", "create task")

	w.Header().Set("Content-Type", "application/json")
	var storageTask *tasks.StorageTask
	err := json.NewDecoder(r.Body).Decode(&storageTask)
	if err != nil {
		log.Errorw("StorageTask json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	task, err := c.db.NewStorageTask(r.Context(), storageTask)
	if err != nil {
		log.Errorw("StorageTask new task", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	taskURL, err := r.URL.Parse("/tasks/" + task.UUID)
	if err != nil {
		log.Errorw("StorageTask parse URL", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", taskURL.String())
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func (c *Controller) newRetrievalTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "create task")
	defer logger.Debugw("request handled", "command", "create task")

	w.Header().Set("Content-Type", "application/json")
	var retrievalTask *tasks.RetrievalTask
	err := json.NewDecoder(r.Body).Decode(&retrievalTask)
	if err != nil {
		log.Errorw("RetrievalTask json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	task, err := c.db.NewRetrievalTask(r.Context(), retrievalTask)
	if err != nil {
		log.Errorw("RetrievalTask new task", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	taskURL, err := r.URL.Parse("/tasks/" + task.UUID)
	if err != nil {
		log.Errorw("RetrievalTask parse URL", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", taskURL.String())
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func (c *Controller) getTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "update task")
	defer logger.Debugw("request handled", "command", "update task")

	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	UUID := vars["uuid"]

	task, err := c.db.Get(r.Context(), UUID)
	if err != nil {
		log.Errorw("get task DB error", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}
