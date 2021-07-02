package controller

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ipld/go-car"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"

	"github.com/filecoin-project/dealbot/controller/state"
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

	dateStart := time.Time{}
	dateEnd := time.Now()
	qargs := r.URL.Query()
	if qstart := qargs.Get("start"); qstart != "" {
		sNum, err := strconv.ParseInt(qstart, 10, 64)
		if err != nil {
			log.Errorw("getTasks failed: parse start", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		dateStart = time.Unix(sNum, 0)
	}
	if qend := qargs.Get("end"); qend != "" {
		eNum, err := strconv.ParseInt(qend, 10, 64)
		if err != nil {
			log.Errorw("getTasks failed: parse end", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		dateEnd = time.Unix(eNum, 0)
	}
	strict := qargs.Get("strict") == "yes"

	taskList, err := c.db.GetAll(r.Context())
	if err != nil {
		log.Errorw("getTasks failed: backend", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	matchList := make([]tasks.Task, 0, len(taskList))
	for _, t := range taskList {
		if t.StartedAt.IsAbsent() {
			if !strict {
				matchList = append(matchList, t)
			}
			continue
		}
		if sa := t.StartedAt.Must().Time(); sa.After(dateStart) && sa.Before(dateEnd) {
			matchList = append(matchList, t)
		}
	}
	tsks := tasks.Type.Tasks.Of(matchList)
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

func (c *Controller) resetWorkerHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "resetWorker")
	defer logger.Debugw("request handled", "command", "resetWorker")

	enableCors(&w, r)
	vars := mux.Vars(r)
	workedBy := vars["workedby"]

	err := c.db.ResetWorkerTasks(r.Context(), workedBy)
	if err != nil {
		log.Errorw("reset worker DB error", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("\"OK\""))
}

func (c *Controller) completeHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "complete")
	defer logger.Debugw("request handled", "command", "complete")

	enableCors(&w, r)
	vars := mux.Vars(r)
	workedBy := vars["workedby"]

	err := c.db.PublishRecordsFrom(r.Context(), workedBy)
	if err != nil {
		log.Errorw("complete worker DB error", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("\"OK\""))
}

func (c *Controller) popTaskHandler(w http.ResponseWriter, r *http.Request) {
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

	if task == nil {
		log.Errorw("get task not found", "uuid", UUID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if _, set := vars["parsed"]; set {
		nilStore := func(_ ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
			return io.Discard, func(_ ipld.Link) error { return nil }, nil
		}
		finished, err := task.Finalize(r.Context(), nilStore, true)
		if err != nil {
			log.Errorw("finalize task error", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		dagjson.Encoder(finished, w)
	} else {
		dagjson.Encoder(task.Representation(), w)
	}
}

func (c *Controller) deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "delete task")
	defer logger.Debugw("request handled", "command", "delete task")

	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	UUID := vars["uuid"]

	err := c.db.Delete(r.Context(), UUID)
	if err != nil {
		log.Errorw("get task DB error", "err", err.Error())
		if err == state.ErrTaskNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) carHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "car")
	defer logger.Debugw("request handled", "command", "car")

	w.Header().Set("Content-Type", "application/octet-stream")

	store := c.db.Store(r.Context())
	rootCid, err := store.Head()
	if err != nil {
		log.Errorw("get task DB error", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)

	ssb := builder.NewSelectorSpecBuilder(basicnode.Prototype__Any{})
	ss := ssb.ExploreRecursive(selector.RecursionLimitNone(), ssb.ExploreUnion(
		ssb.Matcher(),
		ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
	))
	root := car.Dag{
		Root:     rootCid,
		Selector: ss.Node(),
	}
	sc := car.NewSelectiveCar(r.Context(), store, []car.Dag{root})
	err = sc.Write(w)
	if err != nil {
		logger.Info("car write failed", "err", err)
	}
}

func (c *Controller) healthHandler(w http.ResponseWriter, r *http.Request) {
	// basic health check:
	// * do i have a sane database
	if _, err := c.db.GetHead(r.Context()); err != nil {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("ok\nbuilt at: %s", buildDate)))
}

func (c *Controller) sendCORSHeaders(w http.ResponseWriter, r *http.Request) {
	enableCors(&w, r)
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) authHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(fmt.Sprintf("setauth(\"%s\");", strings.TrimSuffix(c.basicauth, "\n"))))
}
