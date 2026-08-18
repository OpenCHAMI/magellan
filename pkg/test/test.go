package test

import (
	"net/http"
)

func Make(response string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// if r.URL.Path != path {
		// 	http.Error(
		// 		w,
		// 		fmt.Sprintf("expected path '%s' but got '%s' instead", path, r.URL.Path),
		// 		http.StatusInternalServerError,
		// 	)
		// 	return
		// }
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}
}
