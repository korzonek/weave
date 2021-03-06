package ipam

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/weave/common"
	"github.com/weaveworks/weave/common/docker"
	"github.com/weaveworks/weave/net/address"
)

func badRequest(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
	common.Log.Warningln("[allocator]:", err.Error())
}

func parseCIDR(w http.ResponseWriter, cidrStr string, net bool) (address.CIDR, bool) {
	cidr, err := address.ParseCIDR(cidrStr)
	if err != nil {
		badRequest(w, err)
		return address.CIDR{}, false
	}
	if net && !cidr.IsSubnet() {
		badRequest(w, fmt.Errorf("Invalid subnet %s - bits after network prefix are not all zero", cidrStr))
		return address.CIDR{}, false
	}
	return cidr, true
}

func writeAddresses(w http.ResponseWriter, cidrs []address.CIDR) {
	for i, cidr := range cidrs {
		fmt.Fprint(w, cidr)
		if i < len(cidrs)-1 {
			w.Write([]byte{' '})
		}
	}
}

func (alloc *Allocator) handleHTTPAllocate(dockerCli *docker.Client, w http.ResponseWriter, ident string, checkAlive bool, subnet address.CIDR) {
	closedChan := w.(http.CloseNotifier).CloseNotify()
	addr, err := alloc.Allocate(ident, subnet,
		func() bool {
			select {
			case <-closedChan:
				return true
			default:
				res := checkAlive && dockerCli != nil && dockerCli.IsContainerNotRunning(ident)
				checkAlive = false // we check only once; if the container dies later we learn about that through events
				return res
			}
		})
	if err != nil {
		if _, ok := err.(*errorCancelled); ok { // cancellation is not really an error
			common.Log.Infoln("[allocator]:", err.Error())
			fmt.Fprint(w, "cancelled")
			return
		}
		badRequest(w, err)
		return
	}

	fmt.Fprintf(w, "%s/%d", addr, subnet.PrefixLen)
}

// HandleHTTP wires up ipams HTTP endpoints to the provided mux.
func (alloc *Allocator) HandleHTTP(router *mux.Router, defaultSubnet address.CIDR, dockerCli *docker.Client) {
	router.Methods("GET").Path("/ipinfo/defaultsubnet").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s", defaultSubnet)
	})

	router.Methods("PUT").Path("/ip/{id}/{ip}/{prefixlen}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if cidr, ok := parseCIDR(w, vars["ip"]+"/"+vars["prefixlen"], false); ok {
			noErrorOnUnknown := r.FormValue("noErrorOnUnknown") == "true"
			if err := alloc.Claim(vars["id"], cidr, noErrorOnUnknown); err != nil {
				badRequest(w, fmt.Errorf("Unable to claim: %s", err))
				return
			}
			w.WriteHeader(204)
		}
	})

	router.Methods("GET").Path("/consensus").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alloc.Consense()
	})

	router.Methods("GET").Path("/ip/{id}/{ip}/{prefixlen}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if subnet, ok := parseCIDR(w, vars["ip"]+"/"+vars["prefixlen"], true); ok {
			cidrs, err := alloc.Lookup(vars["id"], subnet.HostRange())
			if err != nil {
				http.NotFound(w, r)
				return
			}
			writeAddresses(w, cidrs)
		}
	})

	router.Methods("GET").Path("/ip/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addrs, err := alloc.Lookup(mux.Vars(r)["id"], defaultSubnet.HostRange())
		if err != nil {
			http.NotFound(w, r)
			return
		}
		writeAddresses(w, addrs)
	})

	router.Methods("POST").Path("/ip/{id}/{ip}/{prefixlen}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if subnet, ok := parseCIDR(w, vars["ip"]+"/"+vars["prefixlen"], true); ok {
			alloc.handleHTTPAllocate(dockerCli, w, vars["id"], r.FormValue("check-alive") == "true", subnet)
		}
	})

	router.Methods("POST").Path("/ip/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		alloc.handleHTTPAllocate(dockerCli, w, vars["id"], r.FormValue("check-alive") == "true", defaultSubnet)
	})

	router.Methods("DELETE").Path("/ip/{id}/{ip}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		ident := vars["id"]
		ipStr := vars["ip"]
		if ip, err := address.ParseIP(ipStr); err != nil {
			badRequest(w, err)
			return
		} else if err := alloc.Free(ident, ip); err != nil {
			badRequest(w, fmt.Errorf("Unable to free: %s", err))
			return
		}

		w.WriteHeader(204)
	})

	router.Methods("DELETE").Path("/ip/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ident := mux.Vars(r)["id"]
		if err := alloc.Delete(ident); err != nil {
			badRequest(w, err)
			return
		}

		w.WriteHeader(204)
	})

	router.Methods("DELETE").Path("/peer").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alloc.Shutdown()
		w.WriteHeader(204)
	})

	router.Methods("DELETE").Path("/peer/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ident := mux.Vars(r)["id"]
		if err := alloc.AdminTakeoverRanges(ident); err != nil {
			badRequest(w, err)
			return
		}

		w.WriteHeader(204)
	})
}
