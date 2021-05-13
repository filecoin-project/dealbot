package controller

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/ipld/go-ipld-prime/codec/dagjson"

	"github.com/filecoin-project/dealbot/tasks"
)

func enableCors(w *http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	(*w).Header().Set("Access-Control-Allow-Origin", origin)
	(*w).Header().Set("Access-Control-Allow-Headers", "Authorization")
}

func (c *Controller) getTasksHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")

	w.Header().Set("Content-Type", "application/json")

	enableCors(&w, r)

	taskList, err := c.db.GetAll(r.Context())
	if err != nil {
		log.Errorw("getTasks failed: backend", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tsks := tasks.Type.Tasks.Of(taskList)
	dagjson.Encoder(tsks.Representation(), w)
}

func (c *Controller) drainHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "drain")
	defer logger.Debugw("request handled", "command", "drain")

	enableCors(&w, r)
	vars := mux.Vars(r)
	workedBy := vars["workedby"]

	err := c.db.DrainWorker(r.Context(), workedBy)
	if err != nil {
		log.Errorw("drain worker DB error", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("\"OK\""))
}

func (c *Controller) popTaskHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: use a single SQL transaction to remove the need for a mutex here
	c.popTaskLk.Lock()
	defer c.popTaskLk.Unlock()

	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "pop task")
	defer logger.Debugw("request handled", "command", "pop task")

	w.Header().Set("Content-Type", "application/json")
	ptp := tasks.Type.PopTask.NewBuilder()
	err := dagjson.Decoder(ptp, r.Body)
	if err != nil {
		log.Errorw("UpdateTaskRequest json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	task, err := c.db.AssignTask(r.Context(), ptp.Build().(tasks.PopTask))
	if err != nil {
		log.Errorw("popTask failed: backend", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// If none are available, we return a JSON "null".
	if task == nil {
		w.WriteHeader(http.StatusNoContent)
	} else {
		dagjson.Encoder(task.Representation(), w)
	}
}

func (c *Controller) updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "update task")
	defer logger.Debugw("request handled", "command", "update task")

	w.Header().Set("Content-Type", "application/json")
	utp := tasks.Type.UpdateTask.NewBuilder()
	err := dagjson.Decoder(utp, r.Body)
	if err != nil {
		log.Errorw("UpdateTaskRequest json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	uuid := vars["uuid"]
	task, err := c.db.Update(r.Context(), uuid, utp.Build().(tasks.UpdateTask))
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
	dagjson.Encoder(task.Representation(), w)
}

func mustString(s string, _ error) string {
	return s
}

func (c *Controller) newStorageTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "create task")
	defer logger.Debugw("request handled", "command", "create task")

	w.Header().Set("Content-Type", "application/json")
	stp := tasks.Type.StorageTask.NewBuilder()
	if err := dagjson.Decoder(stp, r.Body); err != nil {
		log.Errorw("StorageTask json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	storageTask := stp.Build().(tasks.StorageTask)

	task, err := c.db.NewStorageTask(r.Context(), storageTask)
	if err != nil {
		log.Errorw("StorageTask new task", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	taskURL, err := r.URL.Parse("/tasks/" + mustString(task.UUID.AsString()))
	if err != nil {
		log.Errorw("StorageTask parse URL", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", taskURL.String())
	w.WriteHeader(http.StatusCreated)
	dagjson.Encoder(task.Representation(), w)
}

func (c *Controller) newRetrievalTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "create task")
	defer logger.Debugw("request handled", "command", "create task")

	w.Header().Set("Content-Type", "application/json")

	rtp := tasks.Type.RetrievalTask.NewBuilder()
	if err := dagjson.Decoder(rtp, r.Body); err != nil {
		log.Errorw("RetrievalTask json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	retrievalTask := rtp.Build().(tasks.RetrievalTask)

	task, err := c.db.NewRetrievalTask(r.Context(), retrievalTask)
	if err != nil {
		log.Errorw("RetrievalTask new task", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	taskURL, err := r.URL.Parse("/tasks/" + mustString(task.UUID.AsString()))
	if err != nil {
		log.Errorw("RetrievalTask parse URL", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", taskURL.String())
	w.WriteHeader(http.StatusCreated)
	dagjson.Encoder(task.Representation(), w)
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
	dagjson.Encoder(task.Representation(), w)
}

func (c *Controller) sendCORSHeaders(w http.ResponseWriter, r *http.Request) {
	enableCors(&w, r)
	w.WriteHeader(http.StatusOK)
}
