package graphql

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/filecoin-project/dealbot/controller/state"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/graphql-go/graphql"
)

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

func GetHandler(db state.State) (*http.ServeMux, error) {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"Tasks": &graphql.Field{
					Type: Tasks__type,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						tsks, err := db.GetAll(p.Context)
						if err != nil {
							return nil, err
						}
						return tasks.Type.Tasks.Of(tsks), nil
					},
				},
			},
		}),
	})
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/graphql", CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var result *graphql.Result
		ctx := r.Context()

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
