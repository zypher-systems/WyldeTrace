package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/oui"
)

func (a *API) SuggestDevice(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	portsRaw := r.URL.Query().Get("ports")
	var ports []int
	for _, p := range strings.Split(portsRaw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err == nil {
			ports = append(ports, n)
		}
	}
	httpx.JSON(w, http.StatusOK, oui.SuggestDeviceType(mac, ports, nil))
}
