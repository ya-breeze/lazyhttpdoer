package main

import (
	"log"
	"net/http"

	"github.com/ya-breeze/player/player"
)

func main() {
	c := http.Client{}
	p, err := player.New(&c, "./flows/one", "https://www.google.com", log.Default())
	if err != nil {
		panic(err)
	}

	r, err := http.NewRequest(http.MethodGet, "https://www.google.com/search?q=weather", nil)
	if err != nil {
		panic(err)
	}
	r.Header.Set("User-Agent", "MyUserAgent/1.0")

	res, err := p.Do(r)
	if err != nil {
		panic(err)
	}

	log.Default().Printf("Received response: %d", res.StatusCode)

	res, err = p.Do(r)
	if err != nil {
		panic(err)
	}

	log.Default().Printf("Received response: %d", res.StatusCode)
}
