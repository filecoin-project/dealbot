package graphql

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/filecoin-project/dealbot/controller/state"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/graphql-go/graphql"
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

//go:embed index.html
var index embed.FS

func CorsMiddleware(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// allow cross domain AJAX requests
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
		next(w, r)
	})
}

type postData struct {
	Query     string                 `json:"query"`
	Operation string                 `json:"operation"`
	Variables map[string]interface{} `json:"variables"`
}

func GetHandler(db state.State, accessToken string) (*http.ServeMux, error) {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"Tasks": &graphql.Field{
					Type: Tasks__type,
					Args: graphql.FieldConfigArgument{
						"AccessToken": &graphql.ArgumentConfig{Type: graphql.String, Description: "potentially access-restricted query"},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						if accessToken != "" {
							at, ok := p.Args["AccessToken"]
							if !ok || at.(string) != accessToken {
								return nil, fmt.Errorf("access token required")
							}
						}

						tsks, err := db.GetAll(p.Context)
						if err != nil {
							return nil, err
						}
						return tasks.Type.Tasks.Of(tsks), nil
					},
				},
				"Task": &graphql.Field{
					Type: Task__type,
					Args: graphql.FieldConfigArgument{
						"AccessToken": &graphql.ArgumentConfig{Type: graphql.String, Description: "potentially access-restricted query"},
						"UUID":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String), Description: "task uuid"},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						if accessToken != "" {
							at, ok := p.Args["AccessToken"]
							if !ok || at.(string) != accessToken {
								return nil, fmt.Errorf("access token required")
							}
						}

						uuid := p.Args["UUID"].(string)
						tsk, err := db.Get(p.Context, uuid)
						if err != nil {
							return nil, err
						}
						return tsk, nil
					},
				},
				"FinishedTasks": &graphql.Field{
					Type: FinishedTasks__type,
					Args: graphql.FieldConfigArgument{
						"AccessToken": &graphql.ArgumentConfig{Type: graphql.String, Description: "potentially access-restricted query"},
						"UUIDs":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.String)), Description: "task uuid"},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						if accessToken != "" {
							at, ok := p.Args["AccessToken"]
							if !ok || at.(string) != accessToken {
								return nil, fmt.Errorf("access token required")
							}
						}

						store := db.Store(p.Context)
						storer := func(_ ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
							buf := bytes.Buffer{}
							return &buf, func(l ipld.Link) error {
								c := l.(cidlink.Link).Cid
								return store.Set(c, buf.Bytes())
							}, nil
						}
						uuids := p.Args["UUIDs"].([]interface{})
						finishedTasks := make([]tasks.FinishedTask, 0, len(uuids))
						for _, uuid := range uuids {
							uuidString := uuid.(string)
							tsk, err := db.Get(p.Context, uuidString)
							if err != nil {
								return nil, err
							}
							finishedTask, err := tsk.Finalize(p.Context, storer)
							if err != nil {
								return nil, err
							}
							finishedTasks = append(finishedTasks, finishedTask)
						}
						return finishedTasks, nil
					},
				},
				"RecordUpdate": &graphql.Field{
					Type: RecordUpdate__type,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						hd, err := db.GetHead(p.Context)
						if err != nil {
							return nil, err
						}
						return hd, nil
					},
				},
			},
		}),
	})
	if err != nil {
		return nil, err
	}

	loader := func(ctx context.Context, cl cidlink.Link, builder ipld.NodeBuilder) (ipld.Node, error) {
		store := db.Store(ctx)
		block, err := store.Get(cl.Cid)
		if err != nil {
			return nil, err
		}
		if err := dagjson.Decoder(builder, bytes.NewBuffer(block.RawData())); err != nil {
			return nil, err
		}

		n := builder.Build()
		return n, nil
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(index)))
	mux.Handle("/graphql", CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var result *graphql.Result
		ctx := context.WithValue(r.Context(), nodeLoaderCtxKey, loader)

		if r.Method == "POST" && r.Header.Get("Content-Type") == "application/json" {
			var p postData
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			result = graphql.Do(graphql.Params{
				Context:        ctx,
				Schema:         schema,
				RequestString:  p.Query,
				VariableValues: p.Variables,
				OperationName:  p.Operation,
			})
		} else if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("failed to read req: %v", err)
				return
			}
			result = graphql.Do(graphql.Params{
				Context:       ctx,
				Schema:        schema,
				RequestString: r.Form.Get("query"),
			})
		} else {
			result = graphql.Do(graphql.Params{
				Context:       ctx,
				Schema:        schema,
				RequestString: r.URL.Query().Get("query"),
			})
		}

		if len(result.Errors) > 0 {
			log.Printf("Query had errors: %s, %v", r.URL.Query().Get("query"), result.Errors)
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("Failed to encode response: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))

	return mux, nil
}
