package libollama

import (
	"context"
	"net/http"
	"reflect"

	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/server"
)

func PullModel(ctx context.Context, model string, progress func(status, digest string, completed, total int64)) error {
	// don't blame me for this... it's a workaround.
	// There is a not handled nil case in ollama/server and registryOptions is not exported
	pullModel := reflect.ValueOf(server.PullModel)
	funcType := pullModel.Type()

	// Get the unexported registryOptions type
	registryOptionsType := funcType.In(2)

	// Create a pointer to a new instance of it
	registryOptionsPtr := reflect.New(registryOptionsType.Elem())
	registryOptions := registryOptionsPtr.Elem()

	// Set default values
	registryOptions.FieldByName("Insecure").SetBool(false)
	registryOptions.FieldByName("Username").SetString("")
	registryOptions.FieldByName("Password").SetString("")
	registryOptions.FieldByName("Token").SetString("")
	registryOptions.FieldByName("CheckRedirect").Set(reflect.ValueOf(func(req *http.Request, via []*http.Request) error {
		return nil
	}))

	// Call PullModel with the reflected arguments
	args := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(model),
		reflect.ValueOf(registryOptionsPtr.Interface()),
		reflect.ValueOf(func(pr api.ProgressResponse) {
			progress(pr.Status, pr.Digest, pr.Completed, pr.Total)
		}),
	}

	// Make the call and handle the error result
	out := pullModel.Call(args)
	if err := out[0].Interface(); err != nil {
		return err.(error)
	}
	return nil
}
