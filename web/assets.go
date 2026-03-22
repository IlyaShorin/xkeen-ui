package web

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.html static/*
var embedded embed.FS

func TemplatesFS() fs.FS {
	sub, err := fs.Sub(embedded, "templates")
	if err != nil {
		panic(err)
	}

	return sub
}

func StaticFS() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}

	return sub
}
