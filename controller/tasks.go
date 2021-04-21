package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
)

func (c *Controller) getTasksHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")

	w.Header().Set("Content-Type", "application/json")

	tasks, err := c.db.GetAll(r.Context())
	if err != nil {
		log.Errorw("getTasks failed: backend", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(tasks)
}

func (c *Controller) popTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "pop task")
	defer logger.Debugw("request handled", "command", "pop task")

	w.Header().Set("Content-Type", "application/json")

	// TODO: use a single SQL transaction to query the oldest available
	// task, *and* mark it as in-progress by the worker.
	// This will make pop-task non-racy, preventing the follow-up PATCH to
	// update the status to InProgress and the WorkedBy field.
	// When that happens, pop-task should probably take the dealbot string,
	// to be used in WorkedBy. So perhaps pop-task should be a POST then.
	//
	// For now, to not touch the DB layer, still do a linear search.
	// We still expect that the client will do another pop-task call if
	// another daemon wins the race to self-assigning this task.
	allTasks, err := c.db.GetAll(r.Context())
	if err != nil {
		log.Errorw("getTasks failed: backend", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var firstAvailable *tasks.AuthenticatedTask
	for _, task := range allTasks {
		if task.Status == tasks.Available {
			firstAvailable = task
			break
		}

	}
	// If none are available, we return a JSON "null".
	json.NewEncoder(w).Encode(firstAvailable)
}

func (c *Controller) updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "update task")
	defer logger.Debugw("request handled", "command", "update task")

	w.Header().Set("Content-Type", "application/json")
	var req *client.UpdateTaskRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Errorw("UpdateTaskRequest json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	uuid := vars["uuid"]
	task, err := c.db.Update(r.Context(), uuid, req, c.metricsRecorder)
	if err != nil {
		log.Errorw("UpdateTaskRequest db update", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
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
