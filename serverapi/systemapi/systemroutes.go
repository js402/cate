package systemapi

import (
	"net/http"

	"github.com/js402/cate/serverops"
)

type systemRoutes struct {
	manager serverops.ServiceManager
}

func AddRoutes(mux *http.ServeMux, _ *serverops.Config, manager serverops.ServiceManager) {
	sr := systemRoutes{manager: manager}
	mux.HandleFunc("GET /system/services", sr.info)
}

func (sr *systemRoutes) info(w http.ResponseWriter, r *http.Request) {
	res, err := sr.manager.GetServices()
	if err != nil {
		serverops.Error(w, r, err, serverops.ListOperation)
		return
	}
	serviceNames := []string{}
	for _, sm := range res {
		serviceNames = append(serviceNames, sm.GetServiceName())
	}
	serverops.Encode(w, r, http.StatusOK, serviceNames)
}
