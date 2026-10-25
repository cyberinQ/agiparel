package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func GetSEW01HTML() ([]byte, error) {
	return staticFS.ReadFile("static/sew01.html")
}

func GetPlantViewHTML() ([]byte, error) {
	return staticFS.ReadFile("static/plant_view.html")
}
<<<<<<< HEAD

func GetOnboardingHTML() ([]byte, error) {
	return staticFS.ReadFile("static/onboarding.html")
}

func GetCuttingViewHTML() ([]byte, error) {
	return staticFS.ReadFile("static/cutting_view.html")
=======
func GetOnboardingHTML() ([]byte, error) {
    return staticFS.ReadFile("static/onboarding.html")
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
}