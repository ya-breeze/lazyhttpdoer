package lazyhttpdoer

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
)

const (
	newDirPermissions  = 0o777
	newFilePermissions = 0o664
)

var errorNotFile = errors.New("path is a directory, not a file")

type Player struct {
	client           *http.Client
	requestIndex     int
	path             string
	log              *log.Logger
	shouldUseNetwork bool
	target           *url.URL
}

func New(client *http.Client, dir string, target string, log *log.Logger) (*Player, error) {
	t, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	ok, err := dirExists(dir)
	if err != nil {
		return nil, err
	}
	if !ok {
		log.Printf("Creating dir '%s'", dir)
		err = os.Mkdir(dir, newDirPermissions)
		if err != nil {
			return nil, fmt.Errorf("unable create directory %s: %w", dir, err)
		}
	}

	return &Player{
		client:           client,
		path:             dir,
		log:              log,
		target:           t,
		requestIndex:     0,
		shouldUseNetwork: false,
	}, nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, errorNotFile
		}

		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return false, errors.New("path is a file, not a directory")
		}

		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (p *Player) Do(r *http.Request) (*http.Response, error) {
	// Check if request matches next stored request
	match, res, err := p.matchNextRequest(r)
	if err != nil {
		return nil, err
	}

	if match {
		p.log.Print("Request matches stored sequence - return stored value")
		p.requestIndex++
		return res, nil
	}

	p.log.Print("Request doesn't match stored sequence - need to send to network")
	err = p.replyRequestsIfNecessary()
	if err != nil {
		return nil, err
	}
	err = p.deleteNextRequests()
	if err != nil {
		return nil, err
	}

	p.log.Printf("Sending current request to %s", r.URL)
	res, err = p.client.Do(r)
	if err != nil {
		return nil, err
	}

	err = p.dumpRequestResponse(r, res)
	if err != nil {
		return nil, err
	}

	p.requestIndex++

	return res, nil
}

func (p *Player) deleteNextRequests() error {
	p.log.Print("Deleting all next requests in stored sequence - they are not valid anymore")
	return nil
}

func (p *Player) replyRequestsIfNecessary() error {
	if p.shouldUseNetwork {
		return nil
	}

	if p.requestIndex != 0 {
		p.log.Print("Replying previous requests to put target into desired state")
		for i := 0; i < p.requestIndex; i++ {
			storedReq, _, err := p.readRequestResponse(i)
			if err != nil {
				return err
			}

			fullURL := fmt.Sprintf("%s://%s/%s", p.target.Scheme, p.target.Host, storedReq.URL.String())
			p.log.Printf("Replaying request N%d to %s", i, fullURL)
			r, err := http.NewRequest(storedReq.Method, fullURL, storedReq.Body)
			if err != nil {
				return err
			}

			_, err = p.client.Do(r)
			if err != nil {
				return err
			}
		}
	}
	p.shouldUseNetwork = true

	return nil
}

func (p *Player) matchNextRequest(r *http.Request) (bool, *http.Response, error) {
	storedReq, storedRes, err := p.readRequestResponse(p.requestIndex)
	if err != nil {
		return false, nil, err
	}
	if storedReq == nil || storedRes == nil {
		return false, nil, nil
	}

	storedReq.URL.Scheme = p.target.Scheme
	storedReq.URL.Host = p.target.Host

	if r.Method == storedReq.Method &&
		r.URL.String() == storedReq.URL.String() {
		return true, storedRes, nil
	}

	return false, nil, nil
}

func (p *Player) getRequestFilename(index int) string {
	return path.Join(p.path, fmt.Sprintf("%d.request", index))
}

func (p *Player) getResponseFilename(index int) string {
	return path.Join(p.path, fmt.Sprintf("%d.response", index))
}

func (p *Player) dumpRequestResponse(r *http.Request, res *http.Response) error {
	p.log.Printf("Storing request/response N%d...", p.requestIndex)

	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		return err
	}
	err = os.WriteFile(p.getRequestFilename(p.requestIndex), dump, newFilePermissions)
	if err != nil {
		return err
	}

	dump, err = httputil.DumpResponse(res, true)
	if err != nil {
		return err
	}
	err = os.WriteFile(p.getResponseFilename(p.requestIndex), dump, newFilePermissions)
	if err != nil {
		return err
	}

	return nil
}

func (p *Player) readRequestResponse(index int) (*http.Request, *http.Response, error) {
	p.log.Printf("Reading request/response N%d...", index)

	exists, err := fileExists(p.getRequestFilename(index))
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, nil
	}
	f, err := os.Open(p.getRequestFilename(index))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	req, err := http.ReadRequest(buf)
	if err != nil {
		return nil, nil, err
	}

	exists, err = fileExists(p.getResponseFilename(index))
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, nil
	}
	f, err = os.Open(p.getResponseFilename(index))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	buf = bufio.NewReader(f)
	res, err := http.ReadResponse(buf, req)
	if err != nil {
		return nil, nil, err
	}

	return req, res, nil
}
